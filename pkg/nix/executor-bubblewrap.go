package nix

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

func UseBubbleWrap(profile *Profile) (*ExecutorArgs, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	cwdHash := fmt.Sprintf("%x-%s", md5.Sum([]byte(dir)), filepath.Base(dir))

	bwrap := ExecutorArgs{
		PWD:                        dir,
		NixBinaryMountedPath:       "/nix/bin/nix",
		ProfileFlakeDirMountedPath: "/profile",
		FakeHomeMountedPath:        "/home/nixy",
		NixDirMountedPath:          "/nix",
		WorkspaceDirMountedPath:    "/workspace",
		WorkspaceDirHostPath:       filepath.Join(profile.WorkspacesDir, cwdHash),
	}

	return &bwrap, nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	return false
}

func (nix *Nix) bubblewrapShell(ctx ShellContext) (func(cmd string, args ...string) *exec.Cmd, error) {
	roBinds := []string{
		// "--ro-bind", "/bin", "/bin",
		"--ro-bind", "/etc", "/etc",
		// "--ro-bind-try", "/usr/share/terminfo", "/usr/share/terminfo",
		// "--ro-bind", "/lib", "/lib",
		// "--ro-bind", "/lib64", "/lib64",
		// "--ro-bind", "/run", "/run",
		"--ro-bind", "/usr", "/usr",
		// "--ro-bind", "/var", "/var",
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
		"--bind", nix.profile.FakeHomeDir, nix.executorArgs.FakeHomeMountedPath,
		"--clearenv",
		"--setenv", "HOME", nix.executorArgs.FakeHomeMountedPath,
		"--setenv", "USER", os.Getenv("USER"),
		"--setenv", "TERM", os.Getenv("TERM"),

		// mounts terminfo file, so that your cli tools know and behave according to it
		"--ro-bind", os.Getenv("TERMINFO"), os.Getenv("TERMINFO"),
		"--setenv", "TERMINFO", os.Getenv("TERMINFO"),

		"--setenv", "XDG_SESSION_TYPE", os.Getenv("XDG_SESSION_TYPE"),
		"--setenv", "TERM_PROGRAM", os.Getenv("TERM_PROGRAM"),
		"--setenv", "XDG_CACHE_HOME", filepath.Join(nix.executorArgs.FakeHomeMountedPath, ".cache"),
		"--setenv", "XDG_CONFIG_HOME", filepath.Join(nix.executorArgs.FakeHomeMountedPath, ".config"),
		"--setenv", "XDG_DATA_HOME", filepath.Join(nix.executorArgs.FakeHomeMountedPath, ".local", "share"),
		// nix config
		"--setenv", "NIX_CONFIG", os.Getenv("NIX_CONFIG"),

		// STEP: nixy env vars
		"--setenv", "NIXY_SHELL", os.Getenv("NIXY_SHELL"),
		"--setenv", "NIXY_WORKSPACE_DIR", os.Getenv("NIXY_WORKSPACE_DIR"),
		"--setenv", "NIXY_WORKSPACE_FLAKE_DIR", os.Getenv("NIXY_WORKSPACE_FLAKE_DIR"),

		// STEP: read-write binds
		"--ro-bind", nix.profile.ProfileFlakeDir, nix.executorArgs.ProfileFlakeDirMountedPath,
		// "--tmpfs", nix.executorArgs.WorkspaceDirMountedPath,
		"--bind", nix.executorArgs.WorkspaceDirHostPath, nix.executorArgs.WorkspaceDirMountedPath,

		// Nix Store for nixy bubblewrap shell
		"--bind", nix.profile.NixDir, nix.executorArgs.NixDirMountedPath,
		"--bind", nix.profile.StaticNixBinPath, nix.profile.StaticNixBinPath,

		// Current Working Directory as it is
		"--bind", nix.executorArgs.PWD, nix.executorArgs.PWD,
	}

	// _, mountedWorkspacePath := nix.WorkspaceFlakeDir()

	if !exists(nix.profile.StaticNixBinPath) {
		if err := downloadStaticNixBinary(ctx, nix.profile.StaticNixBinPath); err != nil {
			return nil, err
		}
	}

	// nixShell := []string{
	// 	nix.executorArgs.NixBinaryMountedPath,
	// 	"shell",
	// 	fmt.Sprintf("nixpkgs/%s#bash", nix.NixPkgs),
	// 	"--command",
	// 	"bash",
	// 	"-c",
	// 	strings.Join([]string{
	// 		fmt.Sprintf("PATH=%s:$PATH", filepath.Dir(nix.executorArgs.NixBinaryMountedPath)),
	// 		fmt.Sprintf("cd %s", mountedWorkspacePath),
	// 		fmt.Sprintf("nix develop --quiet --quiet --override-input profile-flake %s --command %s", nix.executorArgs.ProfileFlakeDirMountedPath, program),
	// 	}, "\n"),
	// }

	return func(cmd string, args ...string) *exec.Cmd {
		bwrapArgs = append(bwrapArgs, roBinds...)
		bwrapArgs = append(bwrapArgs, cmd)
		bwrapArgs = append(bwrapArgs, args...)

		return exec.CommandContext(ctx, "bwrap", bwrapArgs...)
	}, nil
}
