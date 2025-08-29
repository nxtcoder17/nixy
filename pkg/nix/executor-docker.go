package nix

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Docker struct {
	profile *Profile

	ProfileFlakeDirMountedPath string
	FakeHomeMountedPath        string
	NixDirMountedPath          string
	WorkspacesDirMountedPath   string
	StaticNixBinMountedPath    string
}

func UseDocker(profile *Profile) (*Docker, error) {
	dockerCfg := Docker{
		profile: profile,

		ProfileFlakeDirMountedPath: "/profile",
		FakeHomeMountedPath:        "/home/nixy",
		NixDirMountedPath:          "/nix",
		WorkspacesDirMountedPath:   "/home/nixy/workspaces",
		StaticNixBinMountedPath:    "/nix/bin/nix",
	}

	return &dockerCfg, nil
}

func (nix *Nix) dockerShell(ctx ShellContext, program string) (*exec.Cmd, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	hostPath, mountedPath := nix.WorkspaceFlakeDir()

	nixShell := []string{
		"nix",
		"shell",
		fmt.Sprintf("nixpkgs/%s#bash", nix.NixPkgs),
		"--command",
		"bash",
		"-c",
		strings.Join([]string{
			fmt.Sprintf("cd %s", mountedPath),
			fmt.Sprintf("nix develop --quiet --quiet --override-input profile-flake %s --command %s", nix.docker.ProfileFlakeDirMountedPath, program),
		}, "\n"),
	}

	// Always use interactive mode for now
	interactiveFlag := "-it"

	addEnv := func(key, value string) string {
		return fmt.Sprintf("%s=%s", key, value)
	}

	addMount := func(src, dest string, flags ...string) string {
		return fmt.Sprintf("%s:%s:%s", src, dest, strings.Join(flags, ","))
	}

	dockerCmd := []string{
		"docker", "run",
		"--hostname", "nixy",
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),

		// STEP: profile flake dir
		"-v", addMount(nix.profile.ProfileFlakeDir, nix.docker.ProfileFlakeDirMountedPath, "z"),

		// Mount Home
		"-v", addMount(nix.profile.FakeHomeDir, nix.docker.FakeHomeMountedPath, "z"),

		// Mount current flake directory
		"-v", addMount(hostPath, mountedPath, "Z"),

		// STEP: Nix Store
		"-v", addMount(nix.profile.NixDir, nix.docker.NixDirMountedPath, "z"),

		// STEP: project dir
		"-v", addMount(cwd, cwd, "Z"),

		"-e", addEnv("HOME", nix.docker.FakeHomeMountedPath),
		"-e", addEnv("USER", ctx.EnvVars["USER"]),
		"-e", addEnv("TERM", ctx.EnvVars["TERM"]),
		"-e", addEnv("TERMINFO", ctx.EnvVars["TERMINFO"]),

		// STEP: mounts terminfo file, so that your cli tools know and behave according to it
		"-v", addMount(ctx.EnvVars["TERMINFO"], ctx.EnvVars["TERMINFO"], "ro", "z"),

		"-e", addEnv("XDG_CACHE_HOME", filepath.Join(nix.docker.FakeHomeMountedPath, ".cache")),
		"-e", addEnv("XDG_CONFIG_HOME", filepath.Join(nix.docker.FakeHomeMountedPath, ".config")),
		"-e", addEnv("XDG_DATA_HOME", filepath.Join(nix.docker.FakeHomeMountedPath, ".local", "share")),

		// nix config
		"-e", addEnv("NIX_CONFIG", ctx.EnvVars["NIX_CONFIG"]),

		// STEP: nixy env vars
		"-e", addEnv("NIXY_SHELL", ctx.EnvVars["NIXY_SHELL"]),
		"-e", addEnv("NIXY_WORKSPACE_DIR", ctx.EnvVars["NIXY_WORKSPACE_DIR"]),
		"-e", addEnv("NIXY_WORKSPACE_FLAKE_DIR", ctx.EnvVars["NIXY_WORKSPACE_FLAKE_DIR"]),
		"--rm", interactiveFlag, "ghcr.io/nxtcoder17/nix:nonroot",
	}

	// dockerCmd = append(dockerCmd, "--")
	dockerCmd = append(dockerCmd, nixShell...)

	return exec.CommandContext(ctx, dockerCmd[0], dockerCmd[1:]...), nil
}
