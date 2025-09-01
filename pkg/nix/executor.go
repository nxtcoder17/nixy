package nix

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func XDGDataDir() string {
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome == "" {
		xdgDataHome = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}

	return filepath.Join(xdgDataHome, "nixy")
}

func (nix *Nix) PrepareShellCommand(ctx ShellContext) (func(cmd string, args ...string) *exec.Cmd, error) {
	switch nix.executor {
	case LocalExecutor:
		return nix.localShell(ctx)
	case DockerExecutor:
		return nix.dockerShell(ctx)
	case BubbleWrapExecutor:
		return nix.bubblewrapShell(ctx)
	default:
		return nil, fmt.Errorf("unknown executor: %s, only local and docker executors are supported", nix.executor)

	}
}
