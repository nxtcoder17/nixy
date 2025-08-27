package nix

import (
	"context"
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

func (nix *Nix) PrepareShellCommand(ctx context.Context, program string) (*exec.Cmd, error) {
	switch nix.executor {
	case LocalExecutor:
		return nix.localShell(ctx, program)
	// case DockerExecutor:
	// 	return nix.prepareDockerCommand(ctx, command, args)
	case BubbleWrapExecutor:
		return nix.bubblewrapShell(ctx, program)
	default:
		return nil, fmt.Errorf("unknown executor: %s, only local and docker executors are supported", nix.executor)

	}
}
