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

func (n *Nixy) nixShellExec(appCtx *app.Context, program string) (*exec.Cmd, error) {
	program = func() string {
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
	}()

	b := []byte(`
experimental-features = flakes nix-command
ignored-acls = security.csm security.selinux system.nfs4_acl com.apple.provenance com.apple.quarantine com.apple.macl com.apple.metadata:kMDItemWhereFroms com.apple.metadata:_kMDItemUserTags com.apple.FinderInfo com.apple.lastuseddate#PS
`)

	if err := os.WriteFile(n.fsPaths.GeneratedNixConfigFilePath, b, 0644); err != nil {
		return nil, err
	}

	// keys := make([]string, 0, len(userEnv))
	// for k := range userEnv {
	// 	keys = append(keys, k)
	// }
	// slices.Sort(keys)

	// for k := range userEnv {
	// 	expanded := os.Expand(
	// 		strings.ReplaceAll(userEnv[k], "$$", "__DOLLOR_ESCAPE__"), func(s string) string {
	// 			if v, ok := executorEnv[s]; ok {
	// 				return v
	// 			}
	// 			return os.Getenv(s)
	// 		},
	// 	)
	// 	userEnv[k] = strings.ReplaceAll(expanded, "__DOLLOR_ESCAPE__", "$")
	// }

	shouldGenNixyArtifacts := true
	if exists(n.fsPaths.GeneratedConfigHashFilePath) {
		b, err := os.ReadFile(n.fsPaths.GeneratedConfigHashFilePath)
		if err != nil {
			return nil, errors.New("failed to read generated nixy config hash file").Wrap(err).KV("hash-file", n.fsPaths.GeneratedConfigHashFilePath)
		}

		shouldGenNixyArtifacts = shouldGenNixyArtifacts || string(b) != n.sha256Sum
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
		scripts = append(scripts,
			// INFO: overwriting config hash file is important, as it ensures nixy re-evaluates incase of an errored previous shell execution
			fmt.Sprintf("printf '' > %s", n.fsPaths.GeneratedConfigHashFilePath),
			// [READ more about nix print-dev-env](https://nix.dev/manual/nix/2.18/command-ref/new-cli/nix3-print-dev-env)
			fmt.Sprintf("nix print-dev-env path:. > %s", shellEnvFileName),
			fmt.Sprintf("printf '%s' > %s", n.sha256Sum, n.fsPaths.GeneratedConfigHashFilePath),
		)
	}

	scripts = append(scripts, fmt.Sprintf("source %s", shellEnvFileName))
	scripts = append(scripts, "exec "+program)

	nixShell := []string{
		"--extra-experimental-features",
		"nix-command",
		"shell",
		fmt.Sprintf("nixpkgs/%s#bash", n.NixPkgs["default"]),
		"--command",
		"bash",
		"-c",
		strings.Join(scripts, "\n"),
	}

	fastlog.Debug("calling executor exec ")
	cmd, err := n.Executor.Exec(appCtx, "nix", nixShell...)
	if err != nil {
		fastlog.Debug("called executor exec ", "error", err)
		return nil, err
	}

	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env,
		"NIXY_SHELL=true",
	)
	// cmd.Env = append(cmd.Env, n.Env.ToEnviron(appCtx)...)

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

	nixyGitRoot := ctx.PWD
	gitRoot, ok := GetGitRootForWorkspace(ctx, ctx.PWD)
	if ok {
		nixyGitRoot = gitRoot
	}
	cmd.Env = append(cmd.Env, "NIXY_GIT_ROOT="+nixyGitRoot)

	fastlog.Debug("Executing", "command", cmd.String())
	defer func() {
		fastlog.Debug("Shell Exited", "in", fmt.Sprintf("%.2fs", time.Since(start).Seconds()))
	}()

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
