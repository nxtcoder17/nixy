package nix

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func UseDocker(profile *Profile) (*ExecutorArgs, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	fakeHomeMountedPath := "/home/nixy"

	dockerCfg := ExecutorArgs{
		PWD:                          dir,
		NixBinaryMountedPath:         "/nixy/nix",
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
				"/nixy",
			},
			NixyWorkspaceDir:      dir,
			NixyWorkspaceFlakeDir: "/workspace",
			NixConfDir:            filepath.Join(profile.FakeHomeDir, ".config", "nix"),
		},
	}

	return &dockerCfg, nil
}

func (nix *Nix) dockerShell(ctx context.Context, command string, args ...string) (*exec.Cmd, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	addMount := func(src, dest string, flags ...string) string {
		return fmt.Sprintf("%s:%s:%s", src, dest, strings.Join(flags, ","))
	}

	dockerCmd := []string{
		"docker", "run",
		"--hostname", "nixy",
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),

		// STEP: profile flake dir
		"-v", addMount(nix.profile.ProfilePath, nix.executorArgs.ProfileDirMountedPath, "z"),

		// Mount Home
		"-v", addMount(nix.profile.FakeHomeDir, nix.executorArgs.FakeHomeMountedPath, "z"),
		"-e", "HOME=" + nix.executorArgs.FakeHomeMountedPath,
		"-e", "PATH=" + strings.Join(nix.executorArgs.EnvVars.Path, ":"),

		// Mount current flake directory
		"-v", addMount(nix.executorArgs.WorkspaceFlakeDirHostPath, nix.executorArgs.WorkspaceFlakeDirMountedPath, "Z"),

		// STEP: nixy and nix binary mounts
		"--tmpfs", "/nixy:ro",
		"--tmpfs", fmt.Sprintf("/bin:rw,uid=%d,gid=%d", os.Getuid(), os.Getgid()),
		"--tmpfs", fmt.Sprintf("/usr:rw,uid=%d,gid=%d", os.Getuid(), os.Getgid()),
		"-v", addMount(nixyEnvVars.NixyBinPath, "/nixy/nixy", "ro", "z"),
		"-v", addMount(nix.profile.StaticNixBinPath, "/nixy/nix", "ro", "z"),

		// STEP: Nix Store
		"-v", addMount(nix.profile.NixDir, nix.executorArgs.NixDirMountedPath, "z"),

		// STEP: project dir
		"-v", addMount(cwd, cwd, "Z"),

		// STEP: mounts terminfo file, so that your cli tools know and behave according to it
		"-v", addMount(nix.executorArgs.EnvVars.TermInfo, nix.executorArgs.EnvVars.TermInfo, "ro", "z"),
	}

	for k, v := range nix.executorArgs.EnvVars.toMap() {
		dockerCmd = append(dockerCmd, "-e", k+"="+v)
	}

	if !exists(nix.profile.StaticNixBinPath) {
		if err := downloadStaticNixBinary(ctx, nix.profile.StaticNixBinPath); err != nil {
			return nil, err
		}
	}

	dockerCmd = append(dockerCmd, "--rm", "-it", "gcr.io/distroless/static-debian12")
	dockerCmd = append(dockerCmd, command)
	dockerCmd = append(dockerCmd, args...)

	return exec.CommandContext(ctx, dockerCmd[0], dockerCmd[1:]...), nil
}
