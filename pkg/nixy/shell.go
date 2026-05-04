package nixy

import (
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nxtcoder17/nixy/pkg/nixy/templates"
)

const (
	shellEnterFileName = "shell-enter.sh"
	shellEnvFileName   = "shell-env.sh"
	configHashFileName = "config.hash"
)

func workspaceGeneratedFilesExist(dir string) bool {
	for _, file := range []string{"flake.nix", shellEnterFileName, shellEnvFileName} {
		if !exists(filepath.Join(dir, file)) {
			return false
		}
	}
	return true
}

// getProfilePackages returns profile packages if NIXY_USE_PROFILE is enabled
func (n *NixyWrapper) getProfilePackages(ctx *Context) []*NormalizedPackage {
	if !ctx.NixyUseProfile || n.profileNixy == nil {
		return nil
	}

	packages := make([]*NormalizedPackage, len(n.profileNixy.Packages))
	for i := range n.profileNixy.Packages {
		pkg := n.profileNixy.Packages[i]
		if pkg.NixPackage != nil {
			// INFO: forces all profile level packages to follow the default from project level nixpkgs
			pkg.NixPackage.Commit = "default"
		}
		packages[i] = pkg
	}
	return packages
}

// getProfileLibraries returns profile libraries if NIXY_USE_PROFILE is enabled
func (n *NixyWrapper) getProfileLibraries(ctx *Context) []string {
	if !ctx.NixyUseProfile || n.profileNixy == nil {
		return nil
	}
	return n.profileNixy.Libraries
}

// getProfileEnvVars returns profile environment variables if NIXY_USE_PROFILE is enabled
func (n *NixyWrapper) getProfileEnvVars(ctx *Context) map[string]string {
	if !ctx.NixyUseProfile || n.profileNixy == nil {
		return nil
	}
	return n.profileNixy.Env
}

func (nix *NixyWrapper) writeWorkspaceFlake(
	ctx *Context, extraPackages []*NormalizedPackage, extraLibraries []string, env map[string]string,
) error {
	if !nix.hasHashChanged {
		slog.Debug("nixy.yml hash has not changed, skipped writing flake.nix")
		return nil
	}

	input := WorkspaceFlakeGenParams{
		NixPkgs:          nix.NixPkgs,
		WorkspaceDirPath: ctx.PWD,
		Packages:         []*NormalizedPackage{},
		Libraries:        []string{},
		Builds:           map[string]Build{},
		EnvVars:          env,
	}

	input.Packages = append(input.Packages, extraPackages...)
	input.Packages = append(input.Packages, nix.Packages...)

	input.Libraries = append(input.Libraries, extraLibraries...)
	input.Libraries = append(input.Libraries, nix.Libraries...)

	maps.Copy(input.Builds, nix.Builds)

	flakeParams, err := genWorkspaceFlakeParams(input)
	if err != nil {
		return err
	}

	shellHook, err := templates.RenderShellEnter(templates.ShellHookParams{
		OnShellEnter: nix.OnShellEnter,
	})
	if err != nil {
		return err
	}

	slog.Debug("writing shell-enter.sh")
	if err := os.WriteFile(filepath.Join(nix.executorArgs.WorkspaceFlakeDirHostPath, shellEnterFileName), []byte(shellHook), 0o744); err != nil {
		return fmt.Errorf("failed to write shell-enter.sh: %w", err)
	}

	flake, err := templates.RenderWorkspaceFlake(flakeParams)
	if err != nil {
		return fmt.Errorf("failed to render flake.nix: %w", err)
	}

	slog.Debug("writing flake.nix")
	return os.WriteFile(filepath.Join(nix.executorArgs.WorkspaceFlakeDirHostPath, "flake.nix"), flake, 0o644)
}

func (n *NixyWrapper) nixShellExec(ctx *Context, program string) (*exec.Cmd, error) {
	// Extract profile-related data (only when NIXY_USE_PROFILE is enabled)
	profilePackages := n.getProfilePackages(ctx)
	profileLibs := n.getProfileLibraries(ctx)
	profileEnvVars := n.getProfileEnvVars(ctx)

	program = func() string {
		if program != "" {
			return program
		}

		if v, ok := n.Env["SHELL"]; ok {
			return v
		}

		if v, ok := os.LookupEnv("SHELL"); ok {
			return filepath.Base(v)
		}

		return "bash"
	}()

	executorEnv := n.executorArgs.EnvVars.toMap(ctx)

	userEnv := make(map[string]string, len(profileEnvVars)+len(n.Env))
	maps.Copy(userEnv, profileEnvVars)
	maps.Copy(userEnv, n.Env)

	keys := make([]string, 0, len(userEnv))
	for k := range userEnv {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for k := range userEnv {
		expanded := os.Expand(
			strings.ReplaceAll(userEnv[k], "$$", "__DOLLOR_ESCAPE__"), func(s string) string {
				if v, ok := executorEnv[s]; ok {
					return v
				}
				return os.Getenv(s)
			},
		)
		userEnv[k] = strings.ReplaceAll(expanded, "__DOLLOR_ESCAPE__", "$")
	}

	if err := n.writeWorkspaceFlake(ctx, profilePackages, profileLibs, userEnv); err != nil {
		return nil, err
	}

	scripts := []string{
		fmt.Sprintf("cd %s", n.executorArgs.WorkspaceFlakeDirMountedPath),
	}

	if n.hasHashChanged {
		scripts = append(scripts,
			// [READ about nix print-dev-env](https://nix.dev/manual/nix/2.18/command-ref/new-cli/nix3-print-dev-env)
			fmt.Sprintf("nix print-dev-env path:. > %s", shellEnvFileName),
		)
	}

	scripts = append(scripts, fmt.Sprintf("source %s", shellEnvFileName))
	scripts = append(scripts, "exec "+program)

	nixShell := []string{"shell"}

	nixShell = append(nixShell,
		fmt.Sprintf("nixpkgs/%s#bash", n.NixPkgs["default"]),
		"--command",
		"bash",
		"-c",
		strings.Join(scripts, "\n"),
	)

	cmd, err := n.PrepareShellCommand(ctx, n.executorArgs.NixBinaryMountedPath, nixShell...)
	if err != nil {
		return nil, err
	}

	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, n.executorArgs.EnvVars.ToEnviron(ctx)...)

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	return cmd, nil
}

func (n *NixyWrapper) Shell(ctx *Context, program string) error {
	start := time.Now()

	n.executorArgs.EnvVars.NixyGitRoot = ctx.PWD

	gitRoot, ok := GetGitRootForWorkspace(ctx, ctx.PWD)
	if ok {
		n.executorArgs.EnvVars.NixyGitRoot = gitRoot
	}

	cmd, err := n.nixShellExec(ctx, program)
	if err != nil {
		return err
	}

	slog.Debug("Executing", "command", cmd.String())
	defer func() {
		slog.Debug("Shell Exited", "in", fmt.Sprintf("%.2fs", time.Since(start).Seconds()))
	}()

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
