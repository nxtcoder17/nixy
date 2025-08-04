package nix

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
)

func (nix *Nix) PackagePaths(ctx context.Context) ([]string, error) {
	args := make([]string, 0, 3+len(nix.Packages))
	args = append(args, "build", "--no-link", "--print-out-paths")
	args = append(args, nix.Packages...)

	b := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, "nix", args...)
	cmd.Stdout = b
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	return strings.Split(b.String(), "\n"), nil
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
