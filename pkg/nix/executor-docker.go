package nix

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func UseDocker(ctx *Context, profile *Profile) (*ExecutorArgs, error) {
	fakeHomeMountedPath := "/home/nixy"

	dockerCfg := ExecutorArgs{
		NixBinaryMountedPath:         "/nixy/nix",
		ProfileDirMountedPath:        "/profile",
		FakeHomeMountedPath:          fakeHomeMountedPath,
		NixDirMountedPath:            "/nix",
		WorkspaceFlakeDirMountedPath: "/workspace",
		WorkspaceFlakeDirHostPath:    deriveWorkspacePath(profile.WorkspacesDir, ctx.PWD),

		EnvVars: executorEnvVars{
			User:           "nixy",
			Home:           fakeHomeMountedPath,
			Term:           os.Getenv("TERM"),
			TermInfo:       os.Getenv("TERMINFO"),
			XDGSessionType: os.Getenv("XDG_SESSION_TYPE"),
			XDGCacheHome:   filepath.Join(fakeHomeMountedPath, ".cache"),
			XDGDataHome:    filepath.Join(fakeHomeMountedPath, ".local", "share"),
			Path: []string{
				"/nixy",
			},
			NixyWorkspaceDir:      ctx.PWD,
			NixyWorkspaceFlakeDir: "/workspace",
			NixConfDir:            filepath.Join(profile.FakeHomeDir, ".config", "nix"),
		},
	}

	return &dockerCfg, nil
}

func (nixy *Nixy) dockerShell(ctx *Context, command string, args ...string) (*exec.Cmd, error) {
	addMount := func(src, dest string, flags ...string) string {
		return fmt.Sprintf("%s:%s:%s", src, dest, strings.Join(flags, ","))
	}

	dockerCmd := []string{
		"docker", "run",
		"--hostname", "nixy",
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),

		// STEP: profile flake dir
		"-v", addMount(nixy.profile.ProfilePath, nixy.executorArgs.ProfileDirMountedPath, "z"),

		// Mount Home
		"-v", addMount(nixy.profile.FakeHomeDir, nixy.executorArgs.FakeHomeMountedPath, "z"),
		"-e", "HOME=" + nixy.executorArgs.FakeHomeMountedPath,
		"-e", "PATH=" + strings.Join(nixy.executorArgs.EnvVars.Path, ":"),

		// Mount current flake directory
		"-v", addMount(nixy.executorArgs.WorkspaceFlakeDirHostPath, nixy.executorArgs.WorkspaceFlakeDirMountedPath, "Z"),

		// STEP: nixy and nix binary mounts
		"--tmpfs", "/nixy:ro",
		"--tmpfs", fmt.Sprintf("/bin:rw,uid=%d,gid=%d", os.Getuid(), os.Getgid()),
		"--tmpfs", fmt.Sprintf("/usr:rw,uid=%d,gid=%d", os.Getuid(), os.Getgid()),
		"-v", addMount(ctx.NixyBinPath, "/nixy/nixy", "ro", "z"),
		"-v", addMount(nixy.profile.StaticNixBinPath, "/nixy/nix", "ro", "z"),

		// STEP: Nix Store
		"-v", addMount(nixy.profile.NixDir, nixy.executorArgs.NixDirMountedPath, "z"),

		// STEP: project dir
		"-v", addMount(nixy.PWD, nixy.PWD, "Z"),

		// STEP: mounts terminfo file, so that your cli tools know and behave according to it
		"-v", addMount(nixy.executorArgs.EnvVars.TermInfo, nixy.executorArgs.EnvVars.TermInfo, "ro", "z"),
	}

	for k, v := range nixy.executorArgs.EnvVars.toMap(ctx) {
		dockerCmd = append(dockerCmd, "-e", k+"="+v)
	}

	if !exists(nixy.profile.StaticNixBinPath) {
		if err := downloadStaticNixBinary(ctx, nixy.profile.StaticNixBinPath); err != nil {
			return nil, err
		}
	}

	dockerCmd = append(dockerCmd, "--rm", "-it", "gcr.io/distroless/static-debian12")
	dockerCmd = append(dockerCmd, command)
	dockerCmd = append(dockerCmd, args...)

	return exec.CommandContext(ctx, dockerCmd[0], dockerCmd[1:]...), nil
}
