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

func (nix *Nix) writeWorkspaceFlake(ctx context.Context) error {
	shouldGenerate := false

	input := WorkspaceFlakeGenParams{
		NixPkgsDefaultCommit: nix.NixPkgs,
		WorkspaceDirPath:     nix.executorArgs.PWD,
		Packages:             []*NormalizedPackage{},
		Libraries:            []string{},
		Builds:               map[string]Build{},
	}

	if nixyEnvVars.NixyUseProfile {
		profileNix, err := LoadFromFile(ctx, nix.profile.ProfileNixyYAMLPath)
		if err != nil {
			return fmt.Errorf("failed to read from profile's nixy.yml: %w", err)
		}
		shouldGenerate = profileNix.hasHashChanged
		input.Packages = append(input.Packages, profileNix.Packages...)
		input.Libraries = append(input.Libraries, profileNix.Libraries...)
	}

	if nix.hasHashChanged {
		shouldGenerate = true
		input.Packages = append(input.Packages, nix.Packages...)
		input.Libraries = append(input.Libraries, nix.Libraries...)
		maps.Copy(input.Builds, nix.Builds)
	}

	if !shouldGenerate {
		return nil
	}

	flakeParams, err := genWorkspaceFlakeParams(input)
	if err != nil {
		return err
	}

	shellHook, err := templates.RenderShellHook(templates.ShellHookParams{
		EnvVars:   nix.executorArgs.EnvVars.toMap(),
		ShellHook: nix.ShellHook,
	})
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(nix.executorArgs.WorkspaceFlakeDirHostPath, "shell-hook.sh"), []byte(shellHook), 0o744); err != nil {
		return fmt.Errorf("failed to write shell-hook.sh: %w", err)
	}

	flake, err := templates.RenderWorkspaceFlake(flakeParams)
	if err != nil {
		return fmt.Errorf("failed to render flake.nix: %w", err)
	}

	return os.WriteFile(filepath.Join(nix.executorArgs.WorkspaceFlakeDirHostPath, "flake.nix"), flake, 0o644)
}

func (n *Nix) nixShellExec(ctx context.Context, program string) (*exec.Cmd, error) {
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

	if n.executorArgs.EnvVars.NixConfDir == "" {
		n.executorArgs.EnvVars.NixConfDir = n.executorArgs.ProfileDirMountedPath
	}

	scripts := []string{}

	for k, v := range n.executorArgs.EnvVars.toMap() {
		scripts = append(scripts, fmt.Sprintf("export %s=%q", k, v))
	}

	for k, v := range n.Env {
		scripts = append(scripts, fmt.Sprintf("export %s=%q", k, v))
	}

	scripts = append(scripts,
		fmt.Sprintf("cd %s", n.executorArgs.WorkspaceFlakeDirMountedPath),
	)

	if n.hasHashChanged || !exists(filepath.Join(n.executorArgs.WorkspaceFlakeDirHostPath, "dev-profile")) {
		scripts = append(scripts,
			fmt.Sprintf("nix develop --profile %s/dev-profile --command %s", n.executorArgs.WorkspaceFlakeDirMountedPath, program),
		)
	} else {
		scripts = append(scripts,
			fmt.Sprintf("nix develop %s/dev-profile --command %s", n.executorArgs.WorkspaceFlakeDirMountedPath, program),
		)
	}

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

func (n *Nix) Shell(ctx context.Context, program string) error {
	cmd, err := n.nixShellExec(ctx, program)
	if err != nil {
		return err
	}

	slog.Debug("Executing", "command", cmd.String())
	defer func() {
		slog.Debug("Shell Exited")
	}()

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
