package nix

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	// "text/template/parse"
	// "strings"
)

type ExecutorOpts struct {
	IsNotTTY bool
}

func XDGDataDir() string {
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome != "" {
		xdgDataHome = filepath.Join(os.Getenv("HOME"), ".local/share")
	}

	return filepath.Join(xdgDataHome, "nixy")
}

func (nix *Nix) PrepareNixCommand(ctx context.Context, command string, args []string, opts ...ExecutorOpts) (*exec.Cmd, error) {
	opt := ExecutorOpts{}
	if len(opts) >= 1 {
		opt = opts[0]
	}

	switch nix.executor {
	case LocalExecutor:
		{
			return exec.CommandContext(ctx, command, args...), nil
		}
	case DockerExecutor:
		{
			dir, err := os.Getwd()
			if err != nil {
				return nil, err
			}

			xdgDataHome := os.Getenv("XDG_DATA_HOME")
			if xdgDataHome == "" {
				xdgDataHome = filepath.Join(os.Getenv("HOME"), ".local/share")
			}

			dataDir := filepath.Join(xdgDataHome, "nixy")

			if err := os.MkdirAll(dataDir, 0o777); err != nil {
				return nil, err
			}

			localDataDir := filepath.Join(dir, ".nixy")
			if err := os.MkdirAll(localDataDir, 0o777); err != nil {
				return nil, err
			}

			// nix.CreateDockerVolumes(nixyNixStore, nixyNixHome)
			interactiveFlag := func() string {
				if opt.IsNotTTY {
					return "-i"
				}
				return "-it"
			}

			dockerCmd := []string{
				"docker", "run",
				// "--cap-drop", "ALL", "--cap-add", "CHOWN", "--cap-add", "SETUID", "--cap-add", "SETGID",
				"-v", fmt.Sprintf("%s:/nix:Z", dataDir),
				"-v", fmt.Sprintf("%s:/root/.cache:Z", localDataDir),
				"--rm", interactiveFlag(), "ghcr.io/nxtcoder17/nix:latest",
			}

			dockerCmd = append(dockerCmd, command)
			dockerCmd = append(dockerCmd, args...)

			return exec.CommandContext(ctx, dockerCmd[0], dockerCmd[1:]...), nil
		}
	case BubbleWrapExecutor:
		pwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}

		roBinds := []string{
			"--ro-bind", "/bin", "/bin",
			"--ro-bind", "/etc", "/etc",
			"--ro-bind", "/lib", "/lib",
			"--ro-bind", "/lib64", "/lib64",
			"--ro-bind", "/run", "/run",
			"--ro-bind", "/usr", "/usr",
			"--ro-bind", "/var", "/var",
		}

		bwrapArgs := []string{
			// no-zombie processes
			"--die-with-parent",
			// "--new-session",

			// share nothing, but the internet for deps downloading
			// "--unshare-user", "--unshare-pid", "--unshare-ipc",
			"--unshare-all",
			"--share-net",

			// for files to have the same UID, and GUID as on the host
			"--uid", fmt.Sprint(os.Geteuid()), "--gid", fmt.Sprint(os.Getegid()),

			// for DNS resolution
			"--ro-bind-try", "/run/systemd/resolve", "/run/systemd/resolve",

			// the usual mounts
			"--proc", "/proc",
			"--dev", "/dev",
			"--tmpfs", "/tmp",

			// Custom User Home for nixy BubbleWrap shell
			"--bind", nix.bubbleWrap.UserHome.HostPath, nix.bubbleWrap.UserHome.MountedPath,
			"--clearenv",
			"--setenv", "HOME", nix.bubbleWrap.UserHome.MountedPath,
			"--setenv", "USER", os.Getenv("USER"),
			"--setenv", "TERM", os.Getenv("TERM"),
			"--setenv", "XDG_SESSION_TYPE", os.Getenv("XDG_SESSION_TYPE"),
			"--setenv", "TERM_PROGRAM", os.Getenv("TERM_PROGRAM"),
			"--setenv", "XDG_CACHE_HOME", filepath.Join(nix.bubbleWrap.UserHome.MountedPath, ".cache"),
			"--setenv", "XDG_CONFIG_HOME", filepath.Join(nix.bubbleWrap.UserHome.MountedPath, ".config"),
			"--setenv", "XDG_DATA_HOME", filepath.Join(nix.bubbleWrap.UserHome.MountedPath, ".local", "share"),
			"--bind", nix.bubbleWrap.ProfileFlakeDir.HostPath, nix.bubbleWrap.ProfileFlakeDir.MountedPath,
			"--bind", nix.bubbleWrap.WorkspacesDir.HostPath, nix.bubbleWrap.WorkspacesDir.MountedPath,
			"--setenv", "NIXY_PROFILE_DIR", nix.bubbleWrap.ProfileFlakeDir.MountedPath,

			// Nix Store for nixy bubblewrap shell
			"--bind", nix.bubbleWrap.NixDir.HostPath, nix.bubbleWrap.NixDir.MountedPath,
			"--dir", "/nix/var/nix",
			"--setenv", "PATH", fmt.Sprintf("/nix/bin:%s", os.Getenv("PATH")),

			// Current Working Directory as it is
			"--bind", pwd, pwd,
		}

		bwrapArgs = append(bwrapArgs, roBinds...)
		bwrapArgs = append(bwrapArgs, command)
		bwrapArgs = append(bwrapArgs, args...)

		return exec.CommandContext(ctx, "bwrap", bwrapArgs...), nil
	default:
		return nil, fmt.Errorf("unknown executor: %s, only local and docker executors are supported", nix.executor)

	}
}
