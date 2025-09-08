package nix

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func XDGDataDir() string {
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome == "" {
		xdgDataHome = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}

	return filepath.Join(xdgDataHome, "nixy")
}

func (nix *Nix) PrepareShellCommand(ctx context.Context, command string, args ...string) (*exec.Cmd, error) {
	switch nix.executor {
	case LocalExecutor:
		return nix.localShell(ctx, command, args...)
	case DockerExecutor:
		return nix.dockerShell(ctx, command, args...)
	case BubbleWrapExecutor:
		return nix.bubblewrapShell(ctx, command, args...)
	default:
		return nil, fmt.Errorf("unknown executor: %s, only local and docker executors are supported", nix.executor)

	}
}

type ExecutorEnvVars struct {
	User                  string   `json:"USER"`
	Home                  string   `json:"HOME"`
	Term                  string   `json:"TERM"`
	TermInfo              string   `json:"TERMINFO"`
	Path                  []string `json:"PATH"`
	XDGSessionType        string   `json:"XDG_SESSION_TYPE"`
	XDGCacheHome          string   `json:"XDG_CACHE_HOME"`
	XDGDataHome           string   `json:"XDG_DATA_HOME"`
	NixyShell             string   `json:"NIXY_SHELL"`
	NixyWorkspaceDir      string   `json:"NIXY_WORKSPACE_DIR"`
	NixyWorkspaceFlakeDir string   `json:"NIXY_WORKSPACE_FLAKE_DIR"`
	NixyBuildHook         string   `json:"NIXY_BUILD_HOOK"`
	NixConfDir            string   `json:"NIX_CONF_DIR"`
}

func (e *ExecutorEnvVars) toMap() map[string]string {
	return map[string]string{
		"USER":             e.User,
		"HOME":             e.Home,
		"TERM":             e.Term,
		"TERMINFO":         e.TermInfo,
		"XDG_SESSION_TYPE": e.XDGSessionType,
		"XDG_CACHE_HOME":   e.XDGCacheHome,
		"XDG_DATA_HOME":    e.XDGDataHome,
		// "NIX_CONFIG":               "experimental-features = nix-command flakes",
		"NIXY_SHELL":               "true",
		"PATH":                     strings.Join(e.Path, ":"),
		"NIXY_WORKSPACE_DIR":       e.NixyWorkspaceDir,
		"NIXY_WORKSPACE_FLAKE_DIR": e.NixyWorkspaceFlakeDir,
		"NIXY_BUILD_HOOK":          e.NixyBuildHook,
		"NIX_CONF_DIR":             e.NixConfDir,
	}
}

func (e *ExecutorEnvVars) ToEnviron() []string {
	m := e.toMap()
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
