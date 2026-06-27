package nixy2

import (
	"fmt"
	"github.com/nxtcoder17/fastlog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/nxtcoder17/go.errors"
	"github.com/nxtcoder17/nixy/internal/app"
	"github.com/nxtcoder17/nixy/pkg/nixy2/templates"
)

type Build struct {
	Packages []*Package `yaml:"packages"`
	Paths    []string   `yaml:"paths"`
	Command  string     `yaml:"command"`
	Dir      string     `yaml:"dir,omitempty"`
}

func (b Build) ResolvedDir() string {
	if b.Dir == "" {
		return "."
	}

	return filepath.Clean(b.Dir)
}

func (b Build) BuildDir(projectDir string) string {
	return filepath.Join(projectDir, b.ResolvedDir())
}

func (b Build) OutputDir(projectDir string) string {
	return filepath.Join(b.BuildDir(projectDir), ".nixy", "dist")
}

func (nw *Nixy) Build(appCtx *app.Context, target string) error {
	build, ok := nw.Builds[target]
	if !ok {
		return errors.New("build target does not exist").KV("target", target)
	}

	b, err := templates.RenderBuildScript(templates.BuildHookParams{
		WorkDir:     build.BuildDir(appCtx.PWD),
		BuildTarget: target,
		OutputDir:   build.OutputDir(appCtx.PWD),
		CopyPaths:   build.Paths,
		Command:     build.Command,
	})
	if err != nil {
		return err
	}

	buildScriptName := "build." + genCleanPathName(target) + ".sh"
	if err := os.WriteFile(filepath.Join(appCtx.FlakePath(), buildScriptName), b, 0o644); err != nil {
		return err
	}

	cmd, err := nw.nixShellExec(appCtx, "echo build successfull")
	if err != nil {
		return err
	}

	fastlog.Debug("Build Executing", "target", target, "command", cmd.String())

	defer func() {
		fastlog.Debug("Shell Exited")
	}()

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func (n *InNixyShell) Build(appCtx *app.Context, target string) error {
	build, ok := n.Builds[target]
	if !ok {
		return errors.New("build target does not exist").KV("target", target)
	}

	b, err := templates.RenderBuildScript(templates.BuildHookParams{
		WorkDir:     filepath.Join(appCtx.PWD, build.Dir),
		BuildTarget: target,
		OutputDir:   filepath.Join(n.fsPaths.GeneratedArtifactsDir, "builds"),
		CopyPaths:   build.Paths,
		Command:     build.Command,
	})
	if err != nil {
		return err
	}

	buildScript := filepath.Join(n.fsPaths.GeneratedArtifactsDir, "scripts", "build"+"-"+genCleanPathName(target))

	if err := os.WriteFile(buildScript, b, 0o644); err != nil {
		return err
	}

	cmd := exec.CommandContext(appCtx, "bash", buildScript)
	cmd.Dir = appCtx.PWD
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	fastlog.Debug("Build Started")
	fastlog.Debug(fmt.Sprintf("[Build %s] Executing", target), "command", cmd.String())

	defer func() {
		fastlog.Debug("Build Finished")
	}()

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
