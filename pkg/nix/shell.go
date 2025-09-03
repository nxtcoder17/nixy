package nix

import (
	"context"
	"fmt"
	"log/slog"
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

func (nix *Nix) writeWorkspaceFlake() error {
	flakeParams, err := nix.GenerateWorkspaceFlakeParams()
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(nix.executorArgs.WorkspaceFlakeDirHostPath, "shell-hook.sh"), []byte(nix.ShellHook), 0o744); err != nil {
		return fmt.Errorf("write shell-hook.sh: %w", err)
	}

	flake, err := templates.RenderWorkspaceFlake(flakeParams)
	if err != nil {
		return fmt.Errorf("render flake.nix: %w", err)
	}
	return os.WriteFile(filepath.Join(nix.executorArgs.WorkspaceFlakeDirHostPath, "flake.nix"), flake, 0o644)
}

func (n *Nix) nixShellExec(ctx context.Context, program string) (*exec.Cmd, error) {
	if err := n.writeWorkspaceFlake(); err != nil {
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

	if n.executorArgs.NixBinaryMountedPath != "nix" {
		scripts = append(scripts, fmt.Sprintf("PATH=%s:$PATH", filepath.Dir(n.executorArgs.NixBinaryMountedPath)))
	}

	scripts = append(scripts,
		fmt.Sprintf("cd %s", n.executorArgs.PWD),
		fmt.Sprintf("nix develop %s --quiet --quiet --override-input profile-flake %s --command %s", n.executorArgs.WorkspaceFlakeDirMountedPath, n.executorArgs.ProfileFlakeDirMountedPath, program),
	)

	nixShell := []string{
		"shell",
		fmt.Sprintf("nixpkgs/%s#bash", n.NixPkgs),
		"--command",
		"bash",
		"-c",
		strings.Join(scripts, "\n"),
	}

	slog.Info("nix shell exec", "command", n.executorArgs.NixBinaryMountedPath)
	cmd, err := n.PrepareShellCommand(ctx, n.executorArgs.NixBinaryMountedPath, nixShell...)
	if err != nil {
		return nil, err
	}

	cmd.Env = n.executorArgs.EnvVars.ToEnviron()
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	return cmd, nil
}

func (n *Nix) Shell(ctx context.Context, program string) error {
	cmd, err := n.nixShellExec(ctx, "")
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
