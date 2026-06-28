package nixy2

import (
	"crypto/sha256"
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

	// Unique container name for the workspace
	containerName := fmt.Sprintf("nixy-%x", sha256.Sum256([]byte(appCtx.PWD)))[:17]

	// Check if the container is already running
	isRunning := false
	inspectCmd := exec.CommandContext(appCtx.Context, "docker", "inspect", "--format", "{{.State.Running}}", containerName)
	if out, err := inspectCmd.Output(); err == nil {
		if strings.TrimSpace(string(out)) == "true" {
			isRunning = true
		}
	}

	if isRunning {
		// Use docker exec to attach to the existing running container
		dockerArgs := []string{
			"exec",
			"-it",
			"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
			"-e", "HOME=" + containerHome,
			"-e", "NIX_CONFIG=ignored-acls = security.csm security.selinux system.nfs4_acl com.apple.provenance com.apple.quarantine com.apple.macl com.apple.metadata:kMDItemWhereFroms com.apple.metadata:_kMDItemUserTags com.apple.FinderInfo com.apple.lastuseddate#PS",
		}

		for k, v := range d.Env {
			dockerArgs = append(dockerArgs, "-e", k+"="+v)
		}

		dockerArgs = append(dockerArgs, containerName, cmd)
		dockerArgs = append(dockerArgs, args...)

		fastlog.Debug("Docker Exec Command Ready", "docker.args", dockerArgs)
		return exec.CommandContext(appCtx.Context, dockerCmd, dockerArgs...), nil
	}

	// Clean up any stopped container with the same name
	_ = exec.CommandContext(appCtx.Context, "docker", "rm", "-f", containerName).Run()

	// Build the docker run command to start the container
	dockerArgs := []string{
		"run",
		"--name", containerName,
		"--hostname", "nixy",
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),

		"-v", addMount(d.NixStoreDockerVolume, "/nix", "z"),
		"-v", addMount(d.HomeDirDockerVolume, "/home", "z"),
		"-e", "HOME=" + containerHome,

		"-v", addMount(appCtx.PWD, appCtx.PWD, "z"),

		// Project Directory is also aliased at /workspace for easier access
		"-v", addMount(appCtx.PWD, "/workspace", "z"),

		"-e", "NIX_CONFIG=ignored-acls = security.csm security.selinux system.nfs4_acl com.apple.provenance com.apple.quarantine com.apple.macl com.apple.metadata:kMDItemWhereFroms com.apple.metadata:_kMDItemUserTags com.apple.FinderInfo com.apple.lastuseddate#PS",

		"-v", addMount(appCtx.NixyExecutableBinPath, "/bin/nixy", "ro", "z"),
	}

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

	dockerArgs = append(dockerArgs, "--rm", "-it", "ghcr.io/nxtcoder17/nix:latest")
	dockerArgs = append(dockerArgs, cmd)
	dockerArgs = append(dockerArgs, args...)

	fastlog.Debug("Docker Command Ready", "docker.args", dockerArgs)
	return exec.CommandContext(appCtx.Context, dockerCmd, dockerArgs...), nil
}
