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

	dockerCmd := []string{
		"docker", "run",
		"--hostname", "nixy",
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),

		// Mount profile directory for flakes and workspaces
		"-v", fmt.Sprintf("%s:%s:z", nix.profile.ProfileFlakeDir, nix.docker.ProfileFlakeDirMountedPath),
		// Mount Home
		"-v", fmt.Sprintf("%s:%s:z", nix.profile.FakeHomeDir, nix.docker.FakeHomeMountedPath),
		// Mount current flake directory
		"-v", fmt.Sprintf("%s:%s:Z", hostPath, mountedPath),

		"-v", fmt.Sprintf("%s:%s:Z", nix.profile.NixDir, nix.docker.NixDirMountedPath),

		// STEP: project dir
		"-v", fmt.Sprintf("%s:%s:Z", cwd, cwd),

		"-e", fmt.Sprintf("HOME=%s", nix.docker.FakeHomeMountedPath),
		"-e", fmt.Sprintf("USER=%s", ctx.EnvVars["USER"]),
		"-e", fmt.Sprintf("TERM=%s", ctx.EnvVars["TERM"]),
		"-e", fmt.Sprintf("TERMINFO=%s", ctx.EnvVars["TERMINFO"]),
		// mounts terminfo file, so that your cli tools know and behave according to it
		"-v", fmt.Sprintf("%s:%s:ro,z", ctx.EnvVars["TERMINFO"], ctx.EnvVars["TERMINFO"]),
		"-e", fmt.Sprintf("XDG_CACHE_HOME=%s", filepath.Join(nix.docker.FakeHomeMountedPath, ".cache")),
		"-e", fmt.Sprintf("XDG_CONFIG_HOME=%s", filepath.Join(nix.docker.FakeHomeMountedPath, ".config")),
		"-e", fmt.Sprintf("XDG_DATA_HOME=%s", filepath.Join(nix.docker.FakeHomeMountedPath, ".local", "share")),

		// nix config
		"-e", fmt.Sprintf("NIX_CONFIG=%s", ctx.EnvVars["NIX_CONFIG"]),

		// STEP: nixy env vars
		"-e", fmt.Sprintf("NIXY_SHELL=%s", ctx.EnvVars["NIXY_SHELL"]),
		"-e", fmt.Sprintf("NIXY_WORKSPACE_DIR=%s", ctx.EnvVars["NIXY_WORKSPACE_DIR"]),
		"-e", fmt.Sprintf("NIXY_WORKSPACE_FLAKE_DIR=%s", ctx.EnvVars["NIXY_WORKSPACE_FLAKE_DIR"]),
		"--rm", interactiveFlag, "ghcr.io/nxtcoder17/nix:nonroot",
	}

	// dockerCmd = append(dockerCmd, "--")
	dockerCmd = append(dockerCmd, nixShell...)

	return exec.CommandContext(ctx, dockerCmd[0], dockerCmd[1:]...), nil
}
