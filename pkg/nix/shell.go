package nix

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nxtcoder17/nixy/pkg/nix/templates"
)

type ShellContext struct {
	context.Context
	EnvVars map[string]string
}

const (
	ShellHookFileName = "shell-hook.sh"
	BuildHookFileName = "build-hook.sh"
)

func (nix *Nixy) writeWorkspaceFlake(ctx *Context) error {
	input := WorkspaceFlakeGenParams{
		NixPkgsDefaultCommit: nix.NixPkgs,
		WorkspaceDirPath:     ctx.PWD,
		Packages:             []*NormalizedPackage{},
		Libraries:            []string{},
		Builds:               map[string]Build{},
	}

	if ctx.NixyUseProfile {
		profileNix, err := LoadFromFile(ctx, nix.profile.ProfileNixyYAMLPath)
		if err != nil {
			return fmt.Errorf("failed to read from profile's nixy.yml: %w", err)
		}
		nix.hasHashChanged = nix.hasHashChanged || profileNix.hasHashChanged
		input.Packages = append(input.Packages, profileNix.Packages...)
		input.Libraries = append(input.Libraries, profileNix.Libraries...)
	}

	if nix.hasHashChanged {
		input.Packages = append(input.Packages, nix.Packages...)
		input.Libraries = append(input.Libraries, nix.Libraries...)
		maps.Copy(input.Builds, nix.Builds)
	}

	if !nix.hasHashChanged {
		slog.Debug("nixy.yml hash has not changed, skipped writing flake.nix")
		return nil
	}

	flakeParams, err := genWorkspaceFlakeParams(input)
	if err != nil {
		return err
	}

	shellHook, err := templates.RenderShellHook(templates.ShellHookParams{
		OnShellEnter: nix.OnShellEnter,
	})
	if err != nil {
		return err
	}

	slog.Debug("writing shell-hook.sh")
	if err := os.WriteFile(filepath.Join(nix.executorArgs.WorkspaceFlakeDirHostPath, "shell-hook.sh"), []byte(shellHook), 0o744); err != nil {
		return fmt.Errorf("failed to write shell-hook.sh: %w", err)
	}

	flake, err := templates.RenderWorkspaceFlake(flakeParams)
	if err != nil {
		return fmt.Errorf("failed to render flake.nix: %w", err)
	}

	slog.Debug("writing flake.nix")
	return os.WriteFile(filepath.Join(nix.executorArgs.WorkspaceFlakeDirHostPath, "flake.nix"), flake, 0o644)
}

func (n *Nixy) nixShellExec(ctx *Context, program string) (*exec.Cmd, error) {
	if err := n.writeWorkspaceFlake(ctx); err != nil {
		return nil, err
	}

	if program == "" {
		if v, ok := os.LookupEnv("SHELL"); ok {
			program = filepath.Base(v)
		} else {
			program = "bash"
		}
	}

	scripts := []string{}

	for k, v := range n.executorArgs.EnvVars.toMap(ctx) {
		scripts = append(scripts, fmt.Sprintf("export %s=%q", k, v))
	}

	for k, v := range n.Env {
		scripts = append(scripts, fmt.Sprintf("export %s=%q", k, v))
	}

	scripts = append(scripts,
		fmt.Sprintf("cd %s", n.executorArgs.WorkspaceFlakeDirMountedPath),
	)

	nixFlakeProfileName := "flake.profile"

	if n.hasHashChanged || !exists(filepath.Join(n.executorArgs.WorkspaceFlakeDirHostPath, nixFlakeProfileName)) {
		scripts = append(scripts,
			fmt.Sprintf("nix profile wipe-history --profile ./%s", nixFlakeProfileName),
			fmt.Sprintf("nix develop --profile ./%s --command echo ''", nixFlakeProfileName),

			// [READ about nix print-dev-env](https://nix.dev/manual/nix/2.18/command-ref/new-cli/nix3-print-dev-env)
			fmt.Sprintf("nix print-dev-env ./%s > shell-init.sh", nixFlakeProfileName),
		)
	}

	scripts = append(scripts, "source shell-init.sh")
	scripts = append(scripts, program)

	nixShell := []string{
		"shell",
		"--ignore-environment",
		"--quiet", "--quiet",
		fmt.Sprintf("nixpkgs/%s#bash", n.NixPkgs),
		"--command",
		"bash",
		"-c",
		strings.Join(scripts, "\n"),
	}

	cmd, err := n.PrepareShellCommand(ctx, n.executorArgs.NixBinaryMountedPath, nixShell...)
	if err != nil {
		return nil, err
	}

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	return cmd, nil
}

func (n *Nixy) Shell(ctx *Context, program string) error {
	start := time.Now()
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
