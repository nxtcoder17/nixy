package nix

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/nxtcoder17/nixy/pkg/nix/templates"
)

func (n *Nix) Build(ctx context.Context, target string) error {
	build, ok := n.Builds[target]
	if !ok {
		return fmt.Errorf("build target (%s) does not exist", target)
	}

	b, err := templates.RenderBuildHook(templates.BuildHookParams{
		ProjectDir:  n.executorArgs.PWD,
		BuildTarget: target,
		CopyPaths:   build.Paths,
	})
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(n.executorArgs.WorkspaceFlakeDirHostPath, BuildHookFileName), b, 0o644); err != nil {
		return err
	}

	n.executorArgs.EnvVars.NixyBuildHook = "true"

	cmd, err := n.nixShellExec(ctx, "echo build successfull")
	if err != nil {
		return err
	}

	slog.Debug(fmt.Sprintf("[Build %s] Executing", target), "command", cmd.String())

	defer func() {
		slog.Debug("Shell Exited")
	}()

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
