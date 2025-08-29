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
func (n *Nix) getPinnedPackages() (nixPkgs []string, packagesMap map[string][]string, librariesMap map[string][]string) {
	nixPkgsSet := make(map[string]struct{}, len(n.Packages)+len(n.Libraries))
	packagesMap = make(map[string][]string, len(n.Packages))
	librariesMap = make(map[string][]string, len(n.Libraries))

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

			if _, ok := nixPkgsSet[commitHash]; !ok {
				nixPkgsSet[commitHash] = struct{}{}
				nixPkgs = append(nixPkgs, commitHash)
			}

			packagesMap[commitHash] = append(packagesMap[commitHash], packageName)
		}
	}

	for _, pkg := range n.Libraries {
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

			if _, ok := nixPkgsSet[commitHash]; !ok {
				nixPkgsSet[commitHash] = struct{}{}
				nixPkgs = append(nixPkgs, commitHash)
			}

			librariesMap[commitHash] = append(librariesMap[commitHash], packageName)
		}
	}

	return nixPkgs, packagesMap, librariesMap
}

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

	// Parse and organize packages by nixpkgs commit
	commitsList, packagesMap, librariesMap := n.getPinnedPackages()

	workspaceDir, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := t.ExecuteTemplate(f, "project-flake", map[string]any{
		"nixpkgsCommitList": commitsList,
		"packagesMap":       packagesMap,
		"librariesMap":      librariesMap,
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
