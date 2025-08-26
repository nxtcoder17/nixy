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

	hostFlakeDir, mountedFlakeDir := n.FlakeDir()

	f, err := os.Create(filepath.Join(hostFlakeDir, "flake.nix"))
	if err != nil {
		return err
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
				return n.bubbleWrap.ProfileFlakeDir.MountedPath
			}
			return ""
		}(),
		"nixpkgsCommit": n.NixPkgs,
	}); err != nil {
		return err
	}

	var nixDevelopArgs []string
	if n.executor == BubbleWrapExecutor {
		nixDevelopArgs = append(nixDevelopArgs, "--override-input", "profile-flake", n.bubbleWrap.ProfileFlakeDir.MountedPath)
		nixDevelopArgs = append(nixDevelopArgs, "2> /dev/null")
	}

	nixDevelopArgs = append(nixDevelopArgs, "--command", shell)

	// Prepare nix develop command
	developCmd := fmt.Sprintf("cd %s && nix develop %s", mountedFlakeDir, strings.Join(nixDevelopArgs, " "))

	slog.Debug("CD", "cmd", developCmd)

	bashPkg := fmt.Sprintf("nixpkgs/%s#bash", n.NixPkgs)

	args := []string{
		"shell", bashPkg, "--command", "bash", "-c", developCmd,
	}

	cmd, err := n.PrepareNixCommand(ctx, "nix", args)
	if err != nil {
		return err
	}

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	cmd.Env = append(os.Environ(), envVars...)

	slog.Debug("Executing", "command", cmd.String())

	defer func() {
		slog.Debug("Shell Exited")
	}()

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
