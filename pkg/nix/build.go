package nix

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/nxtcoder17/nixy/pkg/nix/templates"
)

func (n *Nix) Build(ctx context.Context, target string) error {
	if err := os.WriteFile(filepath.Join(n.executorArgs.WorkspaceDirHostPath, "shell-hook.sh"), []byte(n.ShellHook), 0o744); err != nil {
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

	if err := os.WriteFile(filepath.Join(n.executorArgs.WorkspaceDirHostPath, "flake.nix"), flake, 0o644); err != nil {
		return fmt.Errorf("failed to create flake.nix at path: %s: %w", n.executorArgs.WorkspaceDirHostPath, err)
	}

	envVars := map[string]string{
		"USER":       os.Getenv("USER"),
		"TERM":       os.Getenv("TERM"),
		"TERMINFO":   os.Getenv("TERMINFO"),
		"NIX_CONFIG": "experimental-features = nix-command flakes",

		"NIXY_SHELL":               "true",
		"NIXY_WORKSPACE_DIR":       n.cwd,
		"NIXY_WORKSPACE_FLAKE_DIR": n.executorArgs.WorkspaceDirMountedPath,
	}

	script := []string{}
	if n.executorArgs.NixBinaryMountedPath != "nix" {
		script = append(script, fmt.Sprintf("PATH=%s:$PATH", filepath.Dir(n.executorArgs.NixBinaryMountedPath)))
	}

	build, ok := n.Builds[target]
	if !ok {
		return fmt.Errorf("build target (%s) does not exist", target)
	}

	script = append(script, fmt.Sprintf("cd %s", n.executorArgs.WorkspaceDirMountedPath))

	for _, path := range build.Paths {
		script = append(script, fmt.Sprintf("mkdir -p $(dirname %s) && cp -r %s/%s ./$(dirname %s)", path, n.executorArgs.PWD, path, path))
	}

	script = append(script,
		fmt.Sprintf("cd %s", n.executorArgs.WorkspaceDirMountedPath),
		// "cat flake.nix",
		fmt.Sprintf("nix build .#%s", target),
	)

	nixShell := []string{
		"shell",
		fmt.Sprintf("nixpkgs/%s#busybox", n.NixPkgs),
		fmt.Sprintf("nixpkgs/%s#bash", n.NixPkgs),
		"--command",
		"bash",
		"-c",
		strings.Join(script, "\n"),
	}

	cmdfn, err := n.PrepareShellCommand(ShellContext{
		Context: ctx,
		EnvVars: envVars,
	})
	if err != nil {
		return err
	}

	cmd := cmdfn(n.executorArgs.NixBinaryMountedPath, nixShell...)

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
