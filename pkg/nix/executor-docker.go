package nix

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func (nix *Nix) dockerShell(ctx context.Context, command string, args []string) (*exec.Cmd, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Create necessary directories
	localCacheDir := filepath.Join(dir, ".nixy")
	if err := os.MkdirAll(localCacheDir, 0o777); err != nil {
		return nil, err
	}

	// Get profile workspace directory (host path)
	hostFlakeDir, _ := nix.FlakeDir()

	// Always use interactive mode for now
	interactiveFlag := "-it"

	dockerCmd := []string{
		"docker", "run",
		// Mount profile directory for flakes and workspaces
		"-v", fmt.Sprintf("%s:/profile:Z", nix.profile.ProfilePath),
		// Mount current flake directory
		"-v", fmt.Sprintf("%s:/workspace-flake:Z", hostFlakeDir),
		// Mount local cache
		"-v", fmt.Sprintf("%s:/root/.cache:Z", localCacheDir),
		// Mount current directory
		"-v", fmt.Sprintf("%s:/workspace:Z", dir),
		"-w", "/workspace",
		"--rm", interactiveFlag, "ghcr.io/nxtcoder17/nix:latest",
	}

	dockerCmd = append(dockerCmd, command)
	dockerCmd = append(dockerCmd, args...)

	return exec.CommandContext(ctx, dockerCmd[0], dockerCmd[1:]...), nil
}

