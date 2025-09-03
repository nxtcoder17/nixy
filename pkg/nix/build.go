package nix

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func (n *Nix) Build(ctx context.Context, target string) error {
	build, ok := n.Builds[target]
	if !ok {
		return fmt.Errorf("build target (%s) does not exist", target)
	}

	scripts := make([]string, 0, len(build.Paths))

	for _, path := range build.Paths {
		scripts = append(scripts, fmt.Sprintf("mkdir -p $(dirname %s) && cp -r %s/%s ./$(dirname %s)", path, n.executorArgs.PWD, path, path))
	}

	scripts = append(scripts,
		"set -e",
		"echo i am being sourced",
		"mkdir -p .builds",
		fmt.Sprintf("nix build --quiet --quiet %s#%s -o .builds/%s", n.executorArgs.WorkspaceFlakeDirMountedPath, target, target),
		"echo i am being executed",
		"exit 1",
	)

	if err := os.WriteFile(filepath.Join(n.executorArgs.WorkspaceFlakeDirHostPath, BuildHookFileName), []byte(strings.Join(scripts, "\n")), 0o644); err != nil {
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
