package nixy

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nxtcoder17/nixy/pkg/nixy/templates"
)

func buildScriptFileName(target string) string {
	target = strings.NewReplacer("/", "_", ":", "_").Replace(target)
	return fmt.Sprintf("build.%s.sh", target)
}

func (nixy *NixyWrapper) Build(ctx *Context, target string) error {
	build, ok := nixy.Builds[target]
	if !ok {
		return fmt.Errorf("build target (%s) does not exist", target)
	}

	b, err := templates.RenderBuildScript(templates.BuildHookParams{
		WorkDir:     build.BuildDir(ctx.PWD),
		BuildTarget: target,
		OutputDir:   build.OutputDir(ctx.PWD),
		CopyPaths:   build.Paths,
		Command:     build.Command,
	})
	if err != nil {
		return err
	}

	buildScript := buildScriptFileName(target)
	if err := os.WriteFile(filepath.Join(nixy.executorArgs.WorkspaceFlakeDirHostPath, buildScript), b, 0o644); err != nil {
		return err
	}

	nixy.executorArgs.EnvVars.NixyBuildScript = buildScript
	nixy.useLocalNixy = false

	cmd, err := nixy.nixShellExec(ctx, "echo build successfull", nixShellExecOptions{IncludeLocalConfig: false})
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

func (n *InShellNixy) Build(ctx context.Context, target string) error {
	build, ok := n.Builds[target]
	if !ok {
		return fmt.Errorf("build target (%s) does not exist", target)
	}

	b, err := templates.RenderBuildScript(templates.BuildHookParams{
		WorkDir:     build.BuildDir(n.PWD),
		BuildTarget: target,
		OutputDir:   build.OutputDir(n.PWD),
		CopyPaths:   build.Paths,
		Command:     build.Command,
	})
	if err != nil {
		return err
	}

	wsFlakeDir, ok := os.LookupEnv("NIXY_WORKSPACE_FLAKE_DIR")
	if !ok {
		return fmt.Errorf("NIXY_WORKSPACE_FLAKE_DIR must be set in a nixy shell")
	}

	buildHookScript := filepath.Join(wsFlakeDir, buildScriptFileName(target))

	if err := os.WriteFile(buildHookScript, b, 0o644); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "bash", buildHookScript)
	cmd.Dir = wsFlakeDir
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	slog.Debug("Build Started")
	slog.Debug(fmt.Sprintf("[Build %s] Executing", target), "command", cmd.String())

	defer func() {
		slog.Debug("Build Finished")
	}()

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
