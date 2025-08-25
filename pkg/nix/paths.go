package nix

import (
	"bytes"
	"context"
	"os"
	"strings"
)

func (nix *Nix) PackagePaths(ctx context.Context) ([]string, error) {
	nixCommand := []string{"nix", "build", "--no-link", "--print-out-paths"}
	nixCommand = append(nixCommand, nix.Packages...)

	cmd, err := nix.PrepareNixCommand(ctx, nixCommand[0], nixCommand[1:], ExecutorOpts{IsNotTTY: true})
	if err != nil {
		return nil, err
	}
	b := new(bytes.Buffer)
	cmd.Stdout = b
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	paths := strings.Split(strings.TrimSpace(b.String()), "\n")

	if nix.Executor == DockerExecutor {
		for i := range paths {
			paths[i] = strings.Replace(paths[i], "/nix", "~/.local/share/nixy", 1)
		}
	}

	return paths, nil
}

func (nix *Nix) BinPaths(ctx context.Context) ([]string, error) {
	paths, err := nix.PackagePaths(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(paths))
	for i := range paths {
		result = append(result, paths[i]+"/bin")
	}

	return result, nil
}

func (nix *Nix) LibPaths(ctx context.Context) ([]string, error) {
	// nix-store --query --references $package >> /kl-tmp/libs.list
	nixCommand := []string{"nix-store", "--query", "--references"}
	nixCommand = append(nixCommand, nix.Packages...)

	cmd, err := nix.PrepareNixCommand(ctx, nixCommand[0], nixCommand[1:], ExecutorOpts{IsNotTTY: true})
	if err != nil {
		return nil, err
	}
	b := new(bytes.Buffer)
	cmd.Stdout = b
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	paths := strings.Split(strings.TrimSpace(b.String()), "\n")

	if nix.Executor == DockerExecutor {
		for i := range paths {
			paths[i] = strings.Replace(paths[i], "/nix", "~/.local/share/nixy", 1)
		}
	}

	return paths, nil
}
