package nixy2

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nxtcoder17/fastlog"
	errors "github.com/nxtcoder17/go.errors"
	"github.com/nxtcoder17/nixy/internal/app"
	"github.com/nxtcoder17/nixy/pkg/nixy2/templates"
)

const (
	shellEnterFileName = "shell-enter.sh"
	shellEnvFileName   = "shell-env.sh"
	nixyMergedFileName = "nixy-merged.yml"
	configHashFileName = "config.hash"
)

func workspaceGeneratedFilesExist(dir string) bool {
	for _, file := range []string{"flake.nix", shellEnterFileName, shellEnvFileName, nixyMergedFileName} {
		if !exists(filepath.Join(dir, file)) {
			return false
		}
	}
	return true
}

func (n *Nixy) generateNixyArtifacts(appCtx *app.Context) error {
	input := WorkspaceFlakeGenParams{
		NixPkgs:          n.NixPkgs,
		WorkspaceDirPath: appCtx.PWD,
		Packages:         n.Packages,
		Libraries:        n.Libraries,
		Builds:           n.Builds,
		EnvVars:          n.Env,
		FSPaths:          n.fsPaths,
	}

	flakeParams, err := genWorkspaceFlakeParams(appCtx, input)
	if err != nil {
		return err
	}

	shellHook, err := templates.RenderShellEnter(templates.ShellHookParams{
		OnShellEnter: n.OnShellEnter,
	})
	if err != nil {
		return err
	}

	fastlog.Debug("writing shell-enter hook", "file", n.fsPaths.GeneratedHookOnShellEnterPath)
	if err := os.WriteFile(n.fsPaths.GeneratedHookOnShellEnterPath, []byte(shellHook), 0o744); err != nil {
		return errors.New("failed to write shell enter hook").Wrap(err).KV("file", n.fsPaths.GeneratedHookOnShellEnterPath)
	}

	flake, err := templates.RenderWorkspaceFlake(flakeParams)
	if err != nil {
		return errors.New("failed to render flake.nix").Wrap(err)
	}

	fastlog.Debug("writing flake.nix")
	if err := os.WriteFile(n.fsPaths.GeneratedFlakeNixPath, flake, 0o644); err != nil {
		return errors.New("failed to write flake.nix").Wrap(err).KV("file", n.fsPaths.GeneratedFlakeNixPath)
	}

	return nil
}

func (n *Nixy) writeNixConfig() error {
	b := []byte(`experimental-features = flakes nix-command
ignored-acls = security.csm security.selinux system.nfs4_acl com.apple.provenance com.apple.quarantine com.apple.macl com.apple.metadata:kMDItemWhereFroms com.apple.metadata:_kMDItemUserTags com.apple.FinderInfo com.apple.lastuseddate#PS
`)
	return os.WriteFile(n.fsPaths.GeneratedNixConfigFilePath, b, 0644)
}

func (n *Nixy) needsArtifactRegeneration() (bool, error) {
	if !exists(n.fsPaths.GeneratedConfigHashFilePath) {
		return true, nil
	}

	b, err := os.ReadFile(n.fsPaths.GeneratedConfigHashFilePath)
	if err != nil {
		return false, errors.New("failed to read generated nixy config hash file").
			Wrap(err).
			KV("hash-file", n.fsPaths.GeneratedConfigHashFilePath)
	}

	return string(b) != n.sha256Sum, nil
}

func (n *Nixy) updateConfigHash() string {
	return strings.Join([]string{
		fmt.Sprintf("printf '' > %s", n.fsPaths.GeneratedConfigHashFilePath),
		fmt.Sprintf("nix print-dev-env path:. > %s", shellEnvFileName),
		fmt.Sprintf("printf '%s' > %s", n.sha256Sum, n.fsPaths.GeneratedConfigHashFilePath),
	}, "\n")
}

func (n *Nixy) resolveShellProgram(program string) string {
	if program != "" {
		return program
	}

	if v, ok := n.Env["SHELL"]; ok {
		return v
	}

	if v, ok := os.LookupEnv("SHELL"); ok {
		return filepath.Base(v)
	}

	return "bash"
}

func (n *Nixy) buildNixShellCommand(scripts []string) []string {
	return []string{
		"--extra-experimental-features", "nix-command",
		"shell",
		fmt.Sprintf("nixpkgs/%s#bash", n.NixPkgs["default"]),
		"--command", "bash", "-c",
		strings.Join(scripts, "\n"),
	}
}

func (n *Nixy) attachGitRootEnv(ctx *app.Context, cmd *exec.Cmd) {
	nixyGitRoot := ctx.PWD
	if gitRoot, ok := GetGitRootForWorkspace(ctx, ctx.PWD); ok {
		nixyGitRoot = gitRoot
	}
	cmd.Env = append(cmd.Env, "NIXY_GIT_ROOT="+nixyGitRoot)
}

func (n *Nixy) nixShellExec(appCtx *app.Context, program string) (*exec.Cmd, error) {
	program = n.resolveShellProgram(program)

	if err := n.writeNixConfig(); err != nil {
		return nil, err
	}

	shouldGenNixyArtifacts, err := n.needsArtifactRegeneration()
	if err != nil {
		return nil, err
	}

	if shouldGenNixyArtifacts {
		if err := n.generateNixyArtifacts(appCtx); err != nil {
			return nil, err
		}
	}

	scripts := []string{
		fmt.Sprintf("cd %s", n.fsPaths.GeneratedArtifactsDir),
	}

	if shouldGenNixyArtifacts {
		scripts = append(scripts, n.updateConfigHash())
	}

	scripts = append(scripts,
		fmt.Sprintf("source %s", shellEnvFileName),
		// program,
		"exec "+program,
	)

	nixShell := n.buildNixShellCommand(scripts)

	fastlog.Debug("calling executor exec ")
	cmd, err := n.Executor.Exec(appCtx, "nix", nixShell...)
	if err != nil {
		fastlog.Debug("called executor exec ", "error", err)
		return nil, err
	}

	cmd.Env = append(cmd.Env, os.Environ()...)

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	return cmd, nil
}

func (n *Nixy) Shell(ctx *app.Context, program string) error {
	start := time.Now()

	cmd, err := n.nixShellExec(ctx, program)
	if err != nil {
		return err
	}

	n.attachGitRootEnv(ctx, cmd)

	fastlog.Debug("Executing", "command", cmd.String())
	defer func() {
		fastlog.Debug("Shell Exited", "in", fmt.Sprintf("%.2fs", time.Since(start).Seconds()))
	}()

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
