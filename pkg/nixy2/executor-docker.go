package nixy2

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/nxtcoder17/fastlog"
	"github.com/nxtcoder17/nixy/internal/app"
	fn "github.com/nxtcoder17/nixy/pkg/functions"
)

type DockerModeConfig struct {
	Ports []string `yaml:"ports,omitempty"`
}

type dockerExecutor struct {
	NixStoreDockerVolume string
	HomeDirDockerVolume  string

	Mounts []NixyMount
	Env    map[string]string
	Ports  []containerPort
}

type containerPort struct {
	HostPort      string
	ContainerPort string
}

const AttrReadOnly = "ro"
const AttrSharedVolume = "z"

func (n *Nixy) NewDockerExecutor(appCtx *app.Context, fsPaths *FSPaths) Executor {
	d := &dockerExecutor{
		NixStoreDockerVolume: "nixy-nix-store",
		HomeDirDockerVolume:  "nixy-user-home",
		Mounts:               n.Mounts,
		Env:                  fn.MapMerge(n.Env, commonExecutorEnv(appCtx)),
	}

	for _, v := range n.DockerModeConfig.Ports {
		sp := strings.Split(v, ":")
		if len(sp) > 2 {
			continue
		}
		if len(sp) == 1 {
			d.Ports = append(d.Ports, containerPort{HostPort: sp[0], ContainerPort: sp[0]})
		}
		if len(sp) == 2 {
			d.Ports = append(d.Ports, containerPort{HostPort: sp[0], ContainerPort: sp[1]})
		}
	}

	fastlog.Debug("docker executor", "ports", d.Ports, "n.dockerModeConfig", n.DockerModeConfig)

	return d
}

func (d *dockerExecutor) createNixVolume(appCtx *app.Context) error {
	hasNewVolume := false

	if err := exec.CommandContext(appCtx.Context, "docker", "volume", "inspect", d.NixStoreDockerVolume).Run(); err != nil {
		hasNewVolume = true
		if err := exec.CommandContext(appCtx.Context, "docker", "volume", "create", d.NixStoreDockerVolume).Run(); err != nil {
			return err
		}
	}

	if err := exec.CommandContext(appCtx.Context, "docker", "volume", "inspect", d.HomeDirDockerVolume).Run(); err != nil {
		hasNewVolume = true
		if err := exec.CommandContext(appCtx.Context, "docker", "volume", "create", d.HomeDirDockerVolume).Run(); err != nil {
			return err
		}
	}

	if hasNewVolume {
		uid := os.Getuid()
		gid := os.Getgid()

		initScript := fmt.Sprintf(`
mkdir -p /nix/store /nix/var/nix /home/nixy/.config/nix
if [ ! -f /home/nixy/.config/nix/nix.conf ]; then
  echo "experimental-features = nix-command flakes" > /home/nixy/.config/nix/nix.conf
fi
chown -R %d:%d /nix /home
`, uid, gid)

		out, err := exec.CommandContext(appCtx.Context, "docker", "run", "--rm",
			"--user", "0:0",
			"-v", d.HomeDirDockerVolume+":"+"/home:z",
			"-v", d.NixStoreDockerVolume+":"+"/nix:z",
			appCtx.NixyDockerModeImage,
			"sh", "-c", initScript,
		).CombinedOutput()

		fastlog.Debug("create volume output", "text", string(out))
		if err != nil {
			fastlog.Debug("create volume output", "text", string(out), "err", err)
			return err
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
		fastlog.Debug("GOT", "err", err)
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
			// "-e", "NIX_CONFIG=ignored-acls = security.csm security.selinux system.nfs4_acl com.apple.provenance com.apple.quarantine com.apple.macl com.apple.metadata:kMDItemWhereFroms com.apple.metadata:_kMDItemUserTags com.apple.FinderInfo com.apple.lastuseddate#PS",
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
		"--security-opt",
		"seccomp=unconfined",
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

	for _, p := range d.Ports {
		dockerArgs = append(dockerArgs, "-p", p.HostPort+":"+p.ContainerPort)
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

	dockerArgs = append(dockerArgs, "--rm", "-it", appCtx.NixyDockerModeImage)
	// dockerArgs = append(dockerArgs, "--rm", "-it", "nixos/nix:latest")
	dockerArgs = append(dockerArgs, cmd)
	dockerArgs = append(dockerArgs, args...)

	fastlog.Debug("Docker Command Ready", "docker.args", dockerArgs)
	return exec.CommandContext(appCtx.Context, dockerCmd, dockerArgs...), nil
}
