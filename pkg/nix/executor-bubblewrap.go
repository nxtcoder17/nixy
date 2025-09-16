package nix

import (
	"context"
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

	fakeHomeMountedPath := "/home/nixy"

	bwrap := ExecutorArgs{
		PWD:                          dir,
		NixBinaryMountedPath:         "/nix/bin/nix",
		ProfileDirMountedPath:        "/profile",
		FakeHomeMountedPath:          fakeHomeMountedPath,
		NixDirMountedPath:            "/nix",
		WorkspaceFlakeDirMountedPath: "/workspace",
		WorkspaceFlakeDirHostPath:    deriveWorkspacePath(profile.WorkspacesDir, dir),

		EnvVars: ExecutorEnvVars{
			User:           "nixy",
			Home:           fakeHomeMountedPath,
			Term:           os.Getenv("TERM"),
			TermInfo:       os.Getenv("TERMINFO"),
			XDGSessionType: os.Getenv("XDG_SESSION_TYPE"),
			XDGCacheHome:   filepath.Join(fakeHomeMountedPath, ".cache"),
			XDGDataHome:    filepath.Join(fakeHomeMountedPath, ".local", "share"),
			Path: []string{
				filepath.Dir(profile.StaticNixBinPath),
			},
			NixyShell:             "true",
			NixyWorkspaceDir:      dir,
			NixyWorkspaceFlakeDir: "/workspace",
			NixConfDir:            filepath.Join(profile.FakeHomeDir, ".config", "nix"),
		},
	}

	return &bwrap, nil
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	if err == nil {
		return true
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	return false
}

func (nix *Nix) bubblewrapShell(ctx context.Context, command string, args ...string) (*exec.Cmd, error) {
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

		"--ro-bind", "/etc", "/etc",

		// mounts terminfo file, so that your cli tools know and behave according to it
		"--ro-bind", nix.executorArgs.EnvVars.TermInfo, nix.executorArgs.EnvVars.TermInfo,

		// STEP: read-write binds
		"--ro-bind", nix.profile.ProfilePath, nix.executorArgs.ProfileDirMountedPath,

		// Custom User Home for nixy BubbleWrap shell
		"--bind", nix.profile.FakeHomeDir, nix.executorArgs.FakeHomeMountedPath,
		"--setenv", "HOME", nix.executorArgs.FakeHomeMountedPath,
		"--bind", nix.executorArgs.WorkspaceFlakeDirHostPath, nix.executorArgs.WorkspaceFlakeDirMountedPath,

		// Nix Store for nixy bubblewrap shell
		"--bind", nix.profile.NixDir, nix.executorArgs.NixDirMountedPath,
		"--bind", nix.profile.StaticNixBinPath, nix.profile.StaticNixBinPath,

		// Current Working Directory as it is
		"--bind", nix.executorArgs.PWD, nix.executorArgs.PWD,
		"--clearenv",
	}

	envMap := nix.executorArgs.EnvVars.toMap()
	for k, v := range envMap {
		bwrapArgs = append(bwrapArgs, "--setenv", k, v)
	}

	bwrapArgs = append(bwrapArgs, command)
	bwrapArgs = append(bwrapArgs, args...)

	if !exists(nix.profile.StaticNixBinPath) {
		if err := downloadStaticNixBinary(ctx, nix.profile.StaticNixBinPath); err != nil {
			return nil, err
		}
	}

	return exec.CommandContext(ctx, "bwrap", bwrapArgs...), nil
}
