package nixy

import (
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	errors "github.com/nxtcoder17/go.errors"
	"github.com/nxtcoder17/nixy/pkg/nixy/templates"
	"gopkg.in/yaml.v3"
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

func (n *NixyWrapper) getLocalNixy() *Nixy {
	if !n.useLocalNixy {
		return nil
	}
	return n.localNixy
}

func (n *NixyWrapper) shellNixPkgs() NixPkgsMap {
	nixpkgs := NixPkgsMap{}
	if localNixy := n.getLocalNixy(); localNixy != nil {
		maps.Copy(nixpkgs, localNixy.NixPkgs)
	}
	maps.Copy(nixpkgs, n.NixPkgs)
	return nixpkgs
}

func (n *NixyWrapper) shellPackages(extraPackages []*NormalizedPackage) []*NormalizedPackage {
	packages := []*NormalizedPackage{}
	packages = append(packages, extraPackages...)
	if localNixy := n.getLocalNixy(); localNixy != nil {
		packages = append(packages, localNixy.Packages...)
	}
	packages = append(packages, n.Packages...)
	return packages
}

func (n *NixyWrapper) shellLibraries(extraLibraries []string) []string {
	libraries := []string{}
	libraries = append(libraries, extraLibraries...)
	if localNixy := n.getLocalNixy(); localNixy != nil {
		libraries = append(libraries, localNixy.Libraries...)
	}
	libraries = append(libraries, n.Libraries...)
	return libraries
}

func (n *NixyWrapper) shellOnEnter() string {
	if localNixy := n.getLocalNixy(); localNixy != nil && localNixy.OnShellEnter != "" {
		return strings.TrimRight(localNixy.OnShellEnter, "\n") + "\n" + n.OnShellEnter
	}
	return n.OnShellEnter
}

func (n *NixyWrapper) shellMounts(ctx *Context) []NixyMount {
	mounts := []NixyMount{}
	if localNixy := n.getLocalNixy(); localNixy != nil {
		mounts = append(mounts, localNixy.Mounts...)
	}
	mounts = append(mounts, n.Mounts...)
	return mounts
}

func (n *NixyWrapper) shellEnvVars(ctx *Context) map[string]string {
	userEnv := make(map[string]string, len(n.Env))
	if localNixy := n.getLocalNixy(); localNixy != nil {
		maps.Copy(userEnv, localNixy.Env)
	}
	maps.Copy(userEnv, n.Env)
	return userEnv
}

func (n *NixyWrapper) mergedNixy(ctx *Context, extraPackages []*NormalizedPackage, extraLibraries []string, env map[string]string) *Nixy {
	return &Nixy{
		NixPkgs:      n.shellNixPkgs(),
		Packages:     n.shellPackages(extraPackages),
		Libraries:    n.shellLibraries(extraLibraries),
		Env:          env,
		OnShellEnter: n.shellOnEnter(),
		Builds:       n.Builds,
		Mounts:       n.shellMounts(ctx),
	}
}

func writeMergedNixyYAML(path string, nixy *Nixy) error {
	output, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer output.Close()

	encoder := yaml.NewEncoder(output)
	encoder.SetIndent(2)
	defer encoder.Close()

	return encoder.Encode(nixy)
}

func (nix *NixyWrapper) writeWorkspaceFlake(
	ctx *Context, extraPackages []*NormalizedPackage, extraLibraries []string, env map[string]string, hashState configHashState,
) error {
	if !hashState.HasChanged {
		slog.Debug("nixy.yml hash has not changed, skipped writing flake.nix")
		return nil
	}
	mergedNixy := nix.mergedNixy(ctx, extraPackages, extraLibraries, env)

	input := WorkspaceFlakeGenParams{
		NixPkgs:          mergedNixy.NixPkgs,
		WorkspaceDirPath: ctx.PWD,
		Packages:         mergedNixy.Packages,
		Libraries:        mergedNixy.Libraries,
		Builds:           map[string]Build{},
		EnvVars:          mergedNixy.Env,
	}

	maps.Copy(input.Builds, nix.Builds)

	flakeParams, err := genWorkspaceFlakeParams(input)
	if err != nil {
		return err
	}

	shellHook, err := templates.RenderShellEnter(templates.ShellHookParams{
		OnShellEnter: mergedNixy.OnShellEnter,
	})
	if err != nil {
		return err
	}

	slog.Debug("writing shell-enter.sh")
	if err := os.WriteFile(filepath.Join(nix.executorArgs.WorkspaceFlakeDirHostPath, shellEnterFileName), []byte(shellHook), 0o744); err != nil {
		return errors.New("failed to write shell-enter.sh file").Wrap(err)
	}

	slog.Debug("writing nixy-merged.yml")
	if err := writeMergedNixyYAML(filepath.Join(nix.executorArgs.WorkspaceFlakeDirHostPath, nixyMergedFileName), mergedNixy); err != nil {
		return errors.New("failed to write nixy-merged.yml file").Wrap(err)
	}

	flake, err := templates.RenderWorkspaceFlake(flakeParams)
	if err != nil {
		return fmt.Errorf("failed to render flake.nix: %w", err)
	}

	slog.Debug("writing flake.nix")
	return os.WriteFile(filepath.Join(nix.executorArgs.WorkspaceFlakeDirHostPath, "flake.nix"), flake, 0o644)
}

type nixShellExecOptions struct {
	IncludeLocalConfig bool
}

func (n *NixyWrapper) nixShellExec(ctx *Context, program string, opts nixShellExecOptions) (*exec.Cmd, error) {
	n.Lock()
	defer n.Unlock()

	n.useLocalNixy = opts.IncludeLocalConfig
	defer func() { n.useLocalNixy = false }()

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

	executorEnv := n.executorArgs.EnvVars.toMap(ctx)

	userEnv := n.shellEnvVars(ctx)

	keys := make([]string, 0, len(userEnv))
	for k := range userEnv {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for k := range userEnv {
		expanded := os.Expand(
			strings.ReplaceAll(userEnv[k], "$$", "__DOLLOR_ESCAPE__"), func(s string) string {
				if v, ok := executorEnv[s]; ok {
					return v
				}
				return os.Getenv(s)
			},
		)
		userEnv[k] = strings.ReplaceAll(expanded, "__DOLLOR_ESCAPE__", "$")
	}

	localHash := ""
	if opts.IncludeLocalConfig {
		localHash = n.localHash
	}
	hashState, err := resolveConfigHashState(
		filepath.Join(n.runtimePaths.WorkspaceNixyDir, configHashFileName),
		n.runtimePaths.WorkspaceNixyDir,
		n.projectHash,
		localHash,
	)
	if err != nil {
		return nil, err
	}

	if err := n.writeWorkspaceFlake(ctx, nil, nil, userEnv, hashState); err != nil {
		return nil, err
	}

	scripts := []string{
		fmt.Sprintf("cd %s", n.executorArgs.WorkspaceFlakeDirMountedPath),
	}

	if hashState.HasChanged {
		scripts = append(scripts,
			// INFO: overwriting config hash file is important, as it ensures nixy re-evaluates incase of an errored previous shell execution
			fmt.Sprintf("printf '' > %s", configHashFileName),
			// [READ more about nix print-dev-env](https://nix.dev/manual/nix/2.18/command-ref/new-cli/nix3-print-dev-env)
			fmt.Sprintf("nix print-dev-env path:. > %s", shellEnvFileName),
			fmt.Sprintf("printf '%s' > %s", hashState.Hash, configHashFileName),
		)
	}

	scripts = append(scripts, fmt.Sprintf("source %s", shellEnvFileName))
	scripts = append(scripts, "exec "+program)

	nixShell := []string{"shell"}

	nixShell = append(nixShell,
		fmt.Sprintf("nixpkgs/%s#bash", n.NixPkgs["default"]),
		"--command",
		"bash",
		"-c",
		strings.Join(scripts, "\n"),
	)

	cmd, err := n.prepareShellCommand(ctx, n.executorArgs.NixBinaryMountedPath, nixShell...)
	if err != nil {
		return nil, err
	}

	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, n.executorArgs.EnvVars.ToEnviron(ctx)...)

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	return cmd, nil
}

func (n *NixyWrapper) Shell(ctx *Context, program string) error {
	start := time.Now()

	n.executorArgs.EnvVars.NixyGitRoot = ctx.PWD

	gitRoot, ok := GetGitRootForWorkspace(ctx, ctx.PWD)
	if ok {
		n.executorArgs.EnvVars.NixyGitRoot = gitRoot
	}

	cmd, err := n.nixShellExec(ctx, program, nixShellExecOptions{IncludeLocalConfig: true})
	if err != nil {
		return err
	}

	slog.Debug("Executing", "command", cmd.String())
	defer func() {
		slog.Debug("Shell Exited", "in", fmt.Sprintf("%.2fs", time.Since(start).Seconds()))
	}()

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
