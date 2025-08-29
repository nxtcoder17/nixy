package nix

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"
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

	t := template.New("project-flake")
	if _, err := t.Parse(templateProjectFlake); err != nil {
		return err
	}

	hostFlakeDir, mountedProjectFlakeDir := n.WorkspaceFlakeDir()

	f, err := os.Create(filepath.Join(hostFlakeDir, "flake.nix"))
	if err != nil {
		return fmt.Errorf("failed to crteate flake.nix at path: %s: %w", hostFlakeDir, err)
	}

	// Parse and organize packages by always pinning them against a nixpkgs commit
	pp, err := n.parsePackages()
	if err != nil {
		return err
	}

	workspaceDir, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := t.ExecuteTemplate(f, "project-flake", map[string]any{
		"nixpkgsCommitList": pp.CommitsList,
		"packagesMap":       pp.PackagesMap,
		"librariesMap":      pp.LibrariesMap,
		"projectDir":        workspaceDir,
		"profileDir": func() string {
			if n.executor == BubbleWrapExecutor {
				return n.bubbleWrap.ProfileFlakeDirMountedPath
			}
			return n.profile.ProfileFlakeDir
		}(),
		"nixpkgsDefaultCommit": n.NixPkgs,
		"shellHook":            n.ShellHook,
	}); err != nil {
		return err
	}

	envVars := map[string]string{
		"USER":       os.Getenv("USER"),
		"TERM":       os.Getenv("TERM"),
		"TERMINFO":   os.Getenv("TERMINFO"),
		"NIX_CONFIG": "experimental-features = nix-command flakes",

		"NIXY_SHELL":               "true",
		"NIXY_WORKSPACE_DIR":       workspaceDir,
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
