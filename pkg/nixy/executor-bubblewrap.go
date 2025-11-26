package nixy

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

func UseBubbleWrap(ctx *Context, runtimePaths *RuntimePaths) (*ExecutorArgs, error) {
	fakeHomeMountedPath := "/home/nixy"

	bwrap := ExecutorArgs{
		NixBinaryMountedPath:         "/nix/bin/nix",
		ProfileDirMountedPath:        "/profile",
		FakeHomeMountedPath:          fakeHomeMountedPath,
		NixDirMountedPath:            "/nix",
		WorkspaceFlakeDirMountedPath: WorkspaceFlakeSandboxMountPath,
		WorkspaceFlakeDirHostPath:    deriveWorkspacePath(runtimePaths.WorkspacesDir, ctx.PWD),

		EnvVars: executorEnvVars{
			User:                  "nixy",
			Home:                  fakeHomeMountedPath,
			Term:                  os.Getenv("TERM"),
			TermInfo:              "/terminfo",
			XDGSessionType:        os.Getenv("XDG_SESSION_TYPE"),
			XDGCacheHome:          filepath.Join(fakeHomeMountedPath, ".cache"),
			XDGDataHome:           filepath.Join(fakeHomeMountedPath, ".local", "share"),
			NixyShell:             "true",
			NixyWorkspaceDir:      ctx.PWD,
			NixyWorkspaceFlakeDir: WorkspaceFlakeSandboxMountPath,
			NixConfDir:            filepath.Join(runtimePaths.FakeHomeDir, ".config", "nix"),
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

func (nixy *NixyWrapper) bubblewrapShell(ctx *Context, command string, args ...string) (*exec.Cmd, error) {
	bwrapArgs := []string{
		// no-zombie processes
		// "--clearenv",
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

		// nixy and nix binary mounts
		"--tmpfs", "/nixy",
		"--setenv", "PATH", "/nixy",
		"--tmpfs", "/bin",
		"--tmpfs", "/usr",
		"--ro-bind", nixy.runtimePaths.StaticNixBinPath, "/nixy/nix",
		"--ro-bind", ctx.NixyBinPath, "/nixy/nixy",

		// STEP: read-write binds
		"--ro-bind", nixy.runtimePaths.BasePath, nixy.executorArgs.ProfileDirMountedPath,

		// Custom User Home for nixy BubbleWrap shell
		"--bind", nixy.runtimePaths.FakeHomeDir, nixy.executorArgs.FakeHomeMountedPath,
		"--setenv", "HOME", nixy.executorArgs.FakeHomeMountedPath,
		"--bind", nixy.executorArgs.WorkspaceFlakeDirHostPath, nixy.executorArgs.WorkspaceFlakeDirMountedPath,

		// Nix Store for nixy bubblewrap shell
		"--bind", nixy.runtimePaths.NixDir, nixy.executorArgs.NixDirMountedPath,

		// Current Working Directory as it is
		"--bind", ctx.PWD, ctx.PWD,

		// INFO: it is just to keep the workspace at /workspace in the sandbox
		"--bind", ctx.PWD, WorkspaceDirSandboxMountPath,
	}

	// Mount terminfo if TERMINFO env var is set
	if terminfo := os.Getenv("TERMINFO"); terminfo != "" {
		bwrapArgs = append(bwrapArgs,
			"--tmpfs", nixy.executorArgs.EnvVars.TermInfo,
			"--ro-bind", terminfo, nixy.executorArgs.EnvVars.TermInfo,
		)
	}

	mounts := nixy.Mounts
	if ctx.NixyUseProfile {
		mounts = append(mounts, nixy.profileNixy.Mounts...)
	}

	executorEnv := nixy.executorArgs.EnvVars.toMap(ctx)

	for _, mount := range mounts {
		flag := "--bind"
		if mount.ReadOnly {
			flag = "--ro-bind"
		}

		src := os.ExpandEnv(mount.Source)
		dst := os.Expand(mount.Destination, func(s string) string {
			if v, ok := executorEnv[s]; ok {
				return v
			}

			if v, ok := nixy.Env[s]; ok {
				return v
			}

			if nixy.profileNixy != nil {
				if v, ok := nixy.profileNixy.Env[s]; ok {
					return v
				}
			}

			// Return original $VAR syntax if not found, so errors are visible
			return "$" + s
		})

		if src == "" || dst == "" {
			return nil, fmt.Errorf("mount has empty source or destination: source=%q, dest=%q (original: %q -> %q)", src, dst, mount.Source, mount.Destination)
		}

		bwrapArgs = append(bwrapArgs, flag, src, dst)
	}

	for k, v := range nixy.executorArgs.EnvVars.toMap(ctx) {
		bwrapArgs = append(bwrapArgs, "--setenv", k, v)
	}

	bwrapArgs = append(bwrapArgs, command)
	bwrapArgs = append(bwrapArgs, args...)

	if !exists(nixy.runtimePaths.StaticNixBinPath) {
		if err := downloadStaticNixBinary(ctx, nixy.runtimePaths.StaticNixBinPath); err != nil {
			return nil, err
		}
	}

	return exec.CommandContext(ctx, "bwrap", bwrapArgs...), nil
}
