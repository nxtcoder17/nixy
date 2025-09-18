package nix

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func XDGDataDir() string {
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome == "" {
		xdgDataHome = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}

	return filepath.Join(xdgDataHome, "nixy")
}

func (nixy *Nixy) PrepareShellCommand(ctx *Context, command string, args ...string) (*exec.Cmd, error) {
	switch ctx.NixyMode {
	case LocalExecutor:
		return nixy.localShell(ctx, command, args...)
	case DockerExecutor:
		return nixy.dockerShell(ctx, command, args...)
	case BubbleWrapExecutor:
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

	Path           []string `json:"PATH"`
	XDGSessionType string   `json:"XDG_SESSION_TYPE"`
	XDGCacheHome   string   `json:"XDG_CACHE_HOME"`
	XDGDataHome    string   `json:"XDG_DATA_HOME"`

	NixyOS string `json:"NIXY_OS"`

	// NIXY_ARCH has value like "amd64" or "arm64"
	NixyArch string `json:"NIXY_ARCH"`

	// NIXY_ARCH_FULL has value like "x86_64"
	NixyArchFull string `json:"NIXY_ARCH_FULL"`

	NixyShell             string `json:"NIXY_SHELL"`
	NixyWorkspaceDir      string `json:"NIXY_WORKSPACE_DIR"`
	NixyWorkspaceFlakeDir string `json:"NIXY_WORKSPACE_FLAKE_DIR"`
	NixyBuildHook         string `json:"NIXY_BUILD_HOOK"`
	NixConfDir            string `json:"NIX_CONF_DIR"`
}

func (e *executorEnvVars) toMap(ctx *Context) map[string]string {
	return map[string]string{
		"USER":     e.User,
		"HOME":     e.Home,
		"TERM":     e.Term,
		"TERMINFO": e.TermInfo,

		"XDG_SESSION_TYPE": e.XDGSessionType,
		"XDG_CACHE_HOME":   e.XDGCacheHome,
		"XDG_DATA_HOME":    e.XDGDataHome,

		// Nixy Env Vars
		"NIXY_OS":   runtime.GOOS,
		"NIXY_ARCH": runtime.GOARCH,
		"NIXY_ARCH_FULL": func() string {
			switch runtime.GOARCH {
			case "amd64":
				return "x86_64"
			case "386":
				return "i686"
			case "arm64":
				return "aarch64"
			default:
				return runtime.GOARCH
			}
		}(),

		"NIXY_EXECUTOR":    string(ctx.NixyMode),
		"NIXY_PROFILE":     ctx.NixyProfile,
		"NIXY_USE_PROFILE": fmt.Sprintf("%v", ctx.NixyUseProfile),

		"NIXY_SHELL":               "true",
		"PATH":                     strings.Join(e.Path, ":"),
		"NIXY_WORKSPACE_DIR":       e.NixyWorkspaceDir,
		"NIXY_WORKSPACE_FLAKE_DIR": e.NixyWorkspaceFlakeDir,
		"NIXY_BUILD_HOOK":          e.NixyBuildHook,
		"NIX_CONF_DIR":             e.NixConfDir,
	}
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
