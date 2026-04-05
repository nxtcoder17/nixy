package nixy

import (
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"log/slog"
	"context"
	"bytes"
)

func XDGDataDir() string {
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome == "" {
		xdgDataHome = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}

	return filepath.Join(xdgDataHome, "nixy")
}

// GitWorktreeEnabledWorkspace returns workspace path that needs to be 
// used/mounted in an executor for a functional git bare repository experience
func GitWorktreeEnabledWorkspace(ctx context.Context, dir string) (bool, string, error) {
	workspaceDir := dir

	gitDir, err := exec.CommandContext(ctx, "git", "rev-parse", "--git-common-dir").CombinedOutput()
	gitDir = bytes.TrimSpace(gitDir)
	if err != nil {
		slog.Debug("[CHECK/git-bare-repository] git-common-dir (FAILED)", "stderr", string(gitDir), "err", err)
    return false, workspaceDir, err
  }

	slog.Debug("[CHECK/git-bare-repository] git-common-dir", "dir", string(gitDir))

	gitBareRepoResult, err := exec.CommandContext(ctx, "git", "--git-dir", string(gitDir), "rev-parse", "--is-bare-repository").CombinedOutput()
	gitBareRepoResult = bytes.TrimSpace(gitBareRepoResult)
	if err != nil {
		slog.Error("[CHECK/git-bare-repository] is-bare-repository (FAILED)", "stderr", string(gitBareRepoResult), "err", err) 
    return false, workspaceDir, err
  }

	isWorktree := false
	if string(gitBareRepoResult) == "true" {
		isWorktree = true
		workspaceDir = string(gitDir)
	}

	slog.Debug("[CHECK/git-bare-repository]", "workspace-path", workspaceDir)

	return isWorktree, workspaceDir, nil
}

func (nixy *NixyWrapper) PrepareShellCommand(ctx *Context, command string, args ...string) (*exec.Cmd, error) {
	switch ctx.NixyMode {
	case LocalMode:
		return nixy.localShell(ctx, command, args...)
	case LocalIgnoreEnvMode:
		return nixy.localShell(ctx, command, args...)
	case DockerMode:
		return nixy.dockerShell(ctx, command, args...)
	case BubbleWrapMode:
		return nixy.bubblewrapShell(ctx, command, args...)
	default:
		return nil, fmt.Errorf("unknown executor: %s, only local and docker executors are supported", ctx.NixyMode)

	}
}

type executorEnvVars struct {
	User     string `json:"USER"`
	Home     string `json:"HOME"`
	Term     string `json:"TERM"`
	TermInfo string `json:"TERMINFO"`

	XDGSessionType string `json:"XDG_SESSION_TYPE"`
	XDGCacheHome   string `json:"XDG_CACHE_HOME"`
	XDGDataHome    string `json:"XDG_DATA_HOME"`

	NixyOS string `json:"NIXY_OS"`

	// NIXY_ARCH has value like "amd64" or "arm64"
	NixyArch string `json:"NIXY_ARCH"`

	// NIXY_ARCH_FULL has value like "x86_64"
	NixyArchFull string `json:"NIXY_ARCH_FULL"`

	NixyShell             string `json:"NIXY_SHELL"`
	NixyWorkspaceDir      string `json:"NIXY_WORKSPACE_DIR"`

	// NixyWorkspaceLabel is just a display only alias for NIXY_WORKSPACE_DIR
	// It comes useful in cases of git worktree integrations
	NixyWorkspaceLabel      string `json:"NIXY_WORKSPACE_LABEL"`

	NixyWorkspaceFlakeDir string `json:"NIXY_WORKSPACE_FLAKE_DIR"`
	NixyBuildHook         string `json:"NIXY_BUILD_HOOK"`
	NixConfDir            string `json:"NIX_CONF_DIR"`
}

func (e *executorEnvVars) toMap(ctx *Context) map[string]string {
	m := map[string]string{
		"USER":     e.User,
		"HOME":     e.Home,
		"TERM":     e.Term,
		"TERMINFO": e.TermInfo,

		"XDG_SESSION_TYPE": e.XDGSessionType,
		"XDG_CACHE_HOME":   e.XDGCacheHome,
		"XDG_DATA_HOME":    e.XDGDataHome,

		"NIXY_EXECUTOR":    string(ctx.NixyMode),
		"NIXY_PROFILE":     ctx.NixyProfile,
		"NIXY_USE_PROFILE": fmt.Sprintf("%v", ctx.NixyUseProfile),

		"NIXY_SHELL":               "true",
		"NIXY_WORKSPACE_DIR":       e.NixyWorkspaceDir,
		"NIXY_WORKSPACE_LABEL":     e.NixyWorkspaceLabel,
		"NIXY_WORKSPACE_FLAKE_DIR": e.NixyWorkspaceFlakeDir,
		"NIXY_BUILD_HOOK":          e.NixyBuildHook,
	}

	if e.NixConfDir != "" {
		m["NIX_CONF_DIR"] = e.NixConfDir
	}

	maps.Copy(m, osArchEnv)
	return m
}

func (e *executorEnvVars) ToEnviron(ctx *Context) []string {
	m := e.toMap(ctx)
	result := make([]string, 0, len(m))

	for k, v := range m {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}

	return result
}

type ExecutorMountPath struct {
	HostPath  string
	MountPath string
	ReadOnly  bool
}
