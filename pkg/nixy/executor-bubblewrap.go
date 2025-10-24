package nixy

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

func UseBubbleWrap(ctx *Context, profile *Profile) (*ExecutorArgs, error) {
	fakeHomeMountedPath := "/home/nixy"

	bwrap := ExecutorArgs{
		NixBinaryMountedPath:         "/nix/bin/nix",
		ProfileDirMountedPath:        "/profile",
		FakeHomeMountedPath:          fakeHomeMountedPath,
		NixDirMountedPath:            "/nix",
		WorkspaceFlakeDirMountedPath: WorkspaceFlakeSandboxMountPath,
		WorkspaceFlakeDirHostPath:    deriveWorkspacePath(profile.WorkspacesDir, ctx.PWD),

		EnvVars: executorEnvVars{
			User:                  "nixy",
			Home:                  fakeHomeMountedPath,
			Term:                  os.Getenv("TERM"),
			TermInfo:              os.Getenv("TERMINFO"),
			XDGSessionType:        os.Getenv("XDG_SESSION_TYPE"),
			XDGCacheHome:          filepath.Join(fakeHomeMountedPath, ".cache"),
			XDGDataHome:           filepath.Join(fakeHomeMountedPath, ".local", "share"),
			NixyShell:             "true",
			NixyWorkspaceDir:      ctx.PWD,
			NixyWorkspaceFlakeDir: WorkspaceFlakeSandboxMountPath,
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

func (nixy *Nixy) bubblewrapShell(ctx *Context, command string, args ...string) (*exec.Cmd, error) {
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
		"--ro-bind", nixy.executorArgs.EnvVars.TermInfo, nixy.executorArgs.EnvVars.TermInfo,

		// nixy and nix binary mounts
		"--tmpfs", "/nixy",
		"--setenv", "PATH", "/nixy",
		"--tmpfs", "/bin",
		"--tmpfs", "/usr",
		"--ro-bind", nixy.profile.StaticNixBinPath, "/nixy/nix",
		"--ro-bind", ctx.NixyBinPath, "/nixy/nixy",

		// STEP: read-write binds
		"--ro-bind", nixy.profile.ProfilePath, nixy.executorArgs.ProfileDirMountedPath,

		// Custom User Home for nixy BubbleWrap shell
		"--bind", nixy.profile.FakeHomeDir, nixy.executorArgs.FakeHomeMountedPath,
		"--setenv", "HOME", nixy.executorArgs.FakeHomeMountedPath,
		"--bind", nixy.executorArgs.WorkspaceFlakeDirHostPath, nixy.executorArgs.WorkspaceFlakeDirMountedPath,

		// Nix Store for nixy bubblewrap shell
		"--bind", nixy.profile.NixDir, nixy.executorArgs.NixDirMountedPath,

		// Current Working Directory as it is
		"--bind", ctx.PWD, ctx.PWD,

		// INFO: it is just to keep the workspace at /workspace in the sandbox
		"--bind", ctx.PWD, WorkspaceDirSandboxMountPath,
		// "--clearenv",
	}

	for _, mount := range nixy.Mounts {
		flag := "--bind"
		if mount.ReadOnly {
			flag = "--ro-bind"
		}
		bwrapArgs = append(bwrapArgs, flag, mount.Source, mount.Destination)
	}

	for k, v := range nixy.executorArgs.EnvVars.toMap(ctx) {
		bwrapArgs = append(bwrapArgs, "--setenv", k, v)
	}

	bwrapArgs = append(bwrapArgs, command)
	bwrapArgs = append(bwrapArgs, args...)

	if !exists(nixy.profile.StaticNixBinPath) {
		if err := downloadStaticNixBinary(ctx, nixy.profile.StaticNixBinPath); err != nil {
			return nil, err
		}
	}

	return exec.CommandContext(ctx, "bwrap", bwrapArgs...), nil
}
