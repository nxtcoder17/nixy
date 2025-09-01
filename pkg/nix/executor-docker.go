package nix

import (
	"crypto/md5"
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

	cwdHash := fmt.Sprintf("%x-%s", md5.Sum([]byte(dir)), filepath.Base(dir))

	dockerCfg := ExecutorArgs{
		PWD:                        dir,
		NixBinaryMountedPath:       "nix",
		ProfileFlakeDirMountedPath: "/profile",
		FakeHomeMountedPath:        "/home/nixy",
		NixDirMountedPath:          "/nix",
		WorkspaceDirMountedPath:    "/workspace",
		WorkspaceDirHostPath:       filepath.Join(profile.WorkspacesDir, cwdHash),
	}

	return &dockerCfg, nil
}

func (nix *Nix) dockerShell(ctx ShellContext) (func(cmd string, args ...string) *exec.Cmd, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// wsFlakeHostPath, wsFlakeMountedPath := nix.WorkspaceFlakeDir()
	//
	// nixShell := []string{
	// 	"nix",
	// 	"shell",
	// 	fmt.Sprintf("nixpkgs/%s#bash", nix.NixPkgs),
	// 	"--command",
	// 	"bash",
	// 	"-c",
	// 	strings.Join([]string{
	// 		fmt.Sprintf("cd %s", wsFlakeMountedPath),
	// 		fmt.Sprintf("nix develop --quiet --quiet --override-input profile-flake %s --command %s", nix.executorArgs.ProfileFlakeDirMountedPath, program),
	// 	}, "\n"),
	// }

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
		// "--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),

		// STEP: profile flake dir
		"-v", addMount(nix.profile.ProfileFlakeDir, nix.executorArgs.ProfileFlakeDirMountedPath, "z"),

		// Mount Home
		"-v", addMount(nix.profile.FakeHomeDir, nix.executorArgs.FakeHomeMountedPath, "z"),

		// Mount current flake directory
		"-v", addMount(nix.executorArgs.WorkspaceDirHostPath, nix.executorArgs.WorkspaceDirMountedPath, "Z"),

		// STEP: Nix Store
		"-v", addMount(nix.profile.NixDir, nix.executorArgs.NixDirMountedPath, "z"),

		// STEP: project dir
		"-v", addMount(cwd, cwd, "Z"),

		"-e", addEnv("HOME", nix.executorArgs.FakeHomeMountedPath),
		"-e", addEnv("USER", ctx.EnvVars["USER"]),
		"-e", addEnv("TERM", ctx.EnvVars["TERM"]),
		"-e", addEnv("TERMINFO", ctx.EnvVars["TERMINFO"]),

		// STEP: mounts terminfo file, so that your cli tools know and behave according to it
		"-v", addMount(ctx.EnvVars["TERMINFO"], ctx.EnvVars["TERMINFO"], "ro", "z"),

		"-e", addEnv("XDG_CACHE_HOME", filepath.Join(nix.executorArgs.FakeHomeMountedPath, ".cache")),
		"-e", addEnv("XDG_CONFIG_HOME", filepath.Join(nix.executorArgs.FakeHomeMountedPath, ".config")),
		"-e", addEnv("XDG_DATA_HOME", filepath.Join(nix.executorArgs.FakeHomeMountedPath, ".local", "share")),

		// nix config
		"-e", addEnv("NIX_CONFIG", ctx.EnvVars["NIX_CONFIG"]),

		// STEP: nixy env vars
		"-e", addEnv("NIXY_SHELL", ctx.EnvVars["NIXY_SHELL"]),
		"-e", addEnv("NIXY_WORKSPACE_DIR", ctx.EnvVars["NIXY_WORKSPACE_DIR"]),
		"-e", addEnv("NIXY_WORKSPACE_FLAKE_DIR", ctx.EnvVars["NIXY_WORKSPACE_FLAKE_DIR"]),
		"--rm", interactiveFlag, "ghcr.io/nxtcoder17/nix:nonroot",
	}

	return func(cmd string, args ...string) *exec.Cmd {
		dockerCmd = append(dockerCmd, "--")
		dockerCmd = append(dockerCmd, cmd)
		dockerCmd = append(dockerCmd, args...)
		return exec.CommandContext(ctx, dockerCmd[0], dockerCmd[1:]...)
	}, nil
}
