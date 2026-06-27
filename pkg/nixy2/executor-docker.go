package nixy2

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/nxtcoder17/fastlog"
	"github.com/nxtcoder17/nixy/internal/app"
)

type dockerExecutor struct {
	NixStoreDir string
	HomeDir     string

	Mounts []NixyMount
	Env    map[string]string
}

const AttrReadOnly = "ro"
const AttrSharedVolume = "z"

func (n *Nixy) NewDockerExecutor(appCtx *app.Context, fsPaths *FSPaths) Executor {
	return &dockerExecutor{
		NixStoreDir: fsPaths.NixStoreDir,
		HomeDir:     fsPaths.UserHomeDir,
		Mounts:      n.Mounts,
		Env:         n.Env,
	}
}

func (d *dockerExecutor) Exec(appCtx *app.Context, cmd string, args ...string) (*exec.Cmd, error) {
	addMount := func(src, dest string, flags ...string) string {
		return fmt.Sprintf("%s:%s:%s", src, dest, strings.Join(flags, ","))
	}

	dockerCmd := "docker"

	containerUsername := "nixy"
	containerHome := "/home/" + containerUsername

	dockerArgs := []string{
		// "docker",
		"run",
		"--hostname", "nixy",
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),

		"-v", addMount(d.NixStoreDir, "/nix", "z"),

		"-v", addMount(d.HomeDir, containerHome, "z"),
		"-e", "HOME=" + containerHome,

		"-v", addMount(appCtx.PWD, appCtx.PWD, "z"),

		// Project Directory is also aliased at /workspace for easier access
		"-v", addMount(appCtx.PWD, "/workspace", "z"),

		// step nixy and nix binary mounts
		"--tmpfs", fmt.Sprintf("/bin:rw,uid=%d,gid=%d", os.Getuid(), os.Getgid()),
		"--tmpfs", fmt.Sprintf("/usr:rw,uid=%d,gid=%d", os.Getuid(), os.Getgid()),
		"-v", addMount(appCtx.NixyExecutableBinPath, "/bin/nixy", "ro", "z"),
	}

	// Mount terminfo if TERMINFO env var is set
	if terminfo := os.Getenv("TERMINFO"); terminfo != "" {
		dockerArgs = append(dockerArgs,
			"-v", addMount(terminfo, terminfo, "ro", "z"),
		)
	}

	nixyShellEnvExpander := func(key string) string {
		switch key {
		case "HOME":
			return d.HomeDir
		default:
			return d.Env[key]
		}
	}

	for _, mount := range d.Mounts {
		attrs := []string{AttrSharedVolume}
		if mount.ReadOnly {
			attrs = append(attrs, AttrReadOnly)
		}

		dockerArgs = append(dockerArgs, "-v", addMount(os.ExpandEnv(mount.Source), os.Expand(mount.Destination, nixyShellEnvExpander), attrs...))
	}

	for k, v := range d.Env {
		dockerArgs = append(dockerArgs, "-e", k+"="+v)
	}

	dockerArgs = append(dockerArgs, "--rm", "-it", "gcr.io/distroless/static-debian12")
	dockerArgs = append(dockerArgs, cmd)
	dockerArgs = append(dockerArgs, args...)

	fastlog.Debug("Docker Command Ready", "docker.args", dockerArgs)
	return exec.CommandContext(appCtx.Context, dockerCmd, dockerArgs...), nil
}
