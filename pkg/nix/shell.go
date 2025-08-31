package nix

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/nxtcoder17/nixy/pkg/nix/templates"
)

type ShellContext struct {
	context.Context
	EnvVars map[string]string
}

func (n *Nix) Shell(ctx context.Context, shell string) error {
	if shell == "" {
		if v, ok := os.LookupEnv("SHELL"); ok {
			shell = v
		} else {
			shell = "bash"
		}
	}

	shell = filepath.Base(shell)

	hostFlakeDir, mountedProjectFlakeDir := n.WorkspaceFlakeDir()

	if err := os.WriteFile(filepath.Join(hostFlakeDir, "shell-hook.sh"), []byte(n.ShellHook), 0o744); err != nil {
		return err
	}

	workspaceFlakeParams, err := n.GenerateWorkspaceFlakeParams()
	if err != nil {
		return err
	}

	flake, err := templates.RenderWorkspaceFlake(workspaceFlakeParams)
	if err != nil {
		return fmt.Errorf("failed to write workspace's flake.nix: %w", err)
	}

	if err := os.WriteFile(filepath.Join(hostFlakeDir, "flake.nix"), flake, 0o644); err != nil {
		return fmt.Errorf("failed to crteate flake.nix at path: %s: %w", hostFlakeDir, err)
	}

	envVars := map[string]string{
		"USER":       os.Getenv("USER"),
		"TERM":       os.Getenv("TERM"),
		"TERMINFO":   os.Getenv("TERMINFO"),
		"NIX_CONFIG": "experimental-features = nix-command flakes",

		"NIXY_SHELL":               "true",
		"NIXY_WORKSPACE_DIR":       n.cwd,
		"NIXY_WORKSPACE_FLAKE_DIR": mountedProjectFlakeDir,
	}

	cmd, err := n.PrepareShellCommand(ShellContext{
		Context: ctx,
		EnvVars: envVars,
	}, shell)
	if err != nil {
		return err
	}

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	slog.Debug("Executing", "command", cmd.String())

	defer func() {
		slog.Debug("Shell Exited")
	}()

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
