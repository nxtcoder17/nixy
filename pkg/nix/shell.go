package nix

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// getPinnedPackages parses packages and groups them by nixpkgs commit
func (n *Nix) getPinnedPackages() map[string][]string {
	nixPkgs := make(map[string][]string)

	for _, pkg := range n.Packages {
		// Add default nixpkgs commit if not specified
		if !strings.HasPrefix(pkg, "nixpkgs/") {
			pkg = fmt.Sprintf("nixpkgs/%s#%s", n.NixPkgs, pkg)
		}

		// Parse nixpkgs packages
		if strings.HasPrefix(pkg, "nixpkgs/") {
			sp := strings.Split(pkg, "#")
			if len(sp) != 2 {
				continue
			}

			commitHash := sp[0][len("nixpkgs/"):]
			packageName := sp[1]

			nixPkgs[commitHash] = append(nixPkgs[commitHash], packageName)
		}
	}

	return nixPkgs
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

	// Environment variables to set in the shell
	envVars := []string{
		"IN_NIXY_SHELL=true",
	}

	t := template.New("project-flake")
	if _, err := t.Parse(templateProjectFlake); err != nil {
		return err
	}

	hostFlakeDir, _ := n.FlakeDir()

	f, err := os.Create(filepath.Join(hostFlakeDir, "flake.nix"))
	if err != nil {
		return fmt.Errorf("failed to crteate flake.nix at path: %s: %w", hostFlakeDir, err)
	}

	// Parse and organize packages by nixpkgs commit
	pinnedPkgs := n.getPinnedPackages()

	workspaceDir, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := t.ExecuteTemplate(f, "project-flake", map[string]any{
		"nixPkgs":    pinnedPkgs,
		"projectDir": workspaceDir,
		"profileDir": func() string {
			if n.executor == BubbleWrapExecutor {
				return n.bubbleWrap.ProfileFlakeDirMountedPath
			}
			return n.profile.ProfileFlakeDir
		}(),
		"nixpkgsCommit": n.NixPkgs,
	}); err != nil {
		return err
	}

	cmd, err := n.PrepareShellCommand(ctx, shell)
	if err != nil {
		return err
	}

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(), envVars...)
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
