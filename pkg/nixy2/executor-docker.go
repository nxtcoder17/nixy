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
	NixStoreDockerVolume string
	HomeDirDockerVolume  string

	Mounts []NixyMount
	Env    map[string]string
}

const AttrReadOnly = "ro"
const AttrSharedVolume = "z"

func (n *Nixy) NewDockerExecutor(appCtx *app.Context, fsPaths *FSPaths) Executor {
	return &dockerExecutor{
		NixStoreDockerVolume: "nixy-nix-store",
		HomeDirDockerVolume:  "nixy-user-home",
		Mounts:               n.Mounts,
		Env:                  n.Env,
	}
}

func (d *dockerExecutor) createNixVolume(appCtx *app.Context) error {
	isNewlyCreated := false

	if err := exec.CommandContext(appCtx.Context, "docker", "volume", "inspect", d.NixStoreDockerVolume).Run(); err != nil {
		isNewlyCreated = true
		if err := exec.CommandContext(appCtx.Context, "docker", "volume", "create", d.NixStoreDockerVolume).Run(); err != nil {
			return err
		}
	}

	if err := exec.CommandContext(appCtx.Context, "docker", "volume", "inspect", d.HomeDirDockerVolume).Run(); err != nil {
		isNewlyCreated = true
		if err := exec.CommandContext(appCtx.Context, "docker", "volume", "create", d.HomeDirDockerVolume).Run(); err != nil {
			return err
		}
	}

	if isNewlyCreated {
		if err2 := exec.CommandContext(appCtx.Context, "docker", "run", "--rm",
			"-v", d.HomeDirDockerVolume+":"+"/home:z",
			"-v", d.NixStoreDockerVolume+":"+"/nix:z",
			"ghcr.io/nxtcoder17/nix:latest",
			"sh", "-c", fmt.Sprintf("mkdir -p /nix/store /nix/var/nix /home/nixy && chown -R %d:%d /nix /home/", os.Getuid(), os.Getgid()),
		).Run(); err2 != nil {
			return err2
		}
	}

	return nil
}

func (d *dockerExecutor) Exec(appCtx *app.Context, cmd string, args ...string) (*exec.Cmd, error) {
	addMount := func(src, dest string, flags ...string) string {
		if len(flags) > 0 {
			return fmt.Sprintf("%s:%s:%s", src, dest, strings.Join(flags, ","))
		}
		return src + ":" + dest
	}

	if err := d.createNixVolume(appCtx); err != nil {
		return nil, err
	}

	dockerCmd := "docker"

	containerUsername := "nixy"
	containerHome := "/home/" + containerUsername

	dockerArgs := []string{
		// "docker",
		"run",
		"--hostname", "nixy",
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),

		// "-v", addMount(d.NixStoreDir, "/nix", "z"),
		"-v", addMount(d.NixStoreDockerVolume, "/nix", "z"),

		"-v", addMount(d.HomeDirDockerVolume, "/home", "z"),
		"-e", "HOME=" + containerHome,

		"-v", addMount(appCtx.PWD, appCtx.PWD, "z"),

		// Project Directory is also aliased at /workspace for easier access
		"-v", addMount(appCtx.PWD, "/workspace", "z"),

		"-e", "NIX_CONFIG=ignored-acls = security.csm security.selinux system.nfs4_acl com.apple.provenance com.apple.quarantine com.apple.macl com.apple.metadata:kMDItemWhereFroms com.apple.metadata:_kMDItemUserTags com.apple.FinderInfo com.apple.lastuseddate#PS",

		// step nixy and nix binary mounts
		// "--tmpfs", fmt.Sprintf("/bin:rw,uid=%d,gid=%d", os.Getuid(), os.Getgid()),
		// "--tmpfs", fmt.Sprintf("/usr:rw,uid=%d,gid=%d", os.Getuid(), os.Getgid()),
		"-v", addMount(appCtx.NixyExecutableBinPath, "/bin/nixy", "ro", "z"),
	}

	// Mount terminfo if TERMINFO env var is set
	// if terminfo := os.Getenv("TERMINFO"); terminfo != "" {
	// 	dockerArgs = append(dockerArgs,
	// 		"-v", addMount(terminfo, terminfo, "ro", "z"),
	// 	)
	// }

	nixyShellEnvExpander := func(key string) string {
		switch key {
		case "HOME":
			return containerHome
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

	// dockerArgs = append(dockerArgs, "--rm", "-it", "gcr.io/distroless/static-debian12")
	dockerArgs = append(dockerArgs, "--rm", "-it", "ghcr.io/nxtcoder17/nix:latest")
	// dockerArgs = append(dockerArgs, "--rm", "-it", "nixos/nix")
	dockerArgs = append(dockerArgs, cmd)
	dockerArgs = append(dockerArgs, args...)

	fastlog.Debug("Docker Command Ready", "docker.args", dockerArgs)
	return exec.CommandContext(appCtx.Context, dockerCmd, dockerArgs...), nil
}
