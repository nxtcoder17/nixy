package nix

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

func (nix *Nix) Install(ctx context.Context, pkgs ...string) error {
	args := []string{
		"shell",
		"--log-format",
		"bar-with-logs",
	}
	args = append(args, nix.Packages...)

	for _, pkg := range pkgs {
		if strings.HasPrefix(pkg, "nixpkgs#") {
		}

		if !strings.HasPrefix(pkg, "nixpkgs/") {
		}
	}

	args = append(args, pkgs...)

	cmd := exec.CommandContext(ctx, "nix", append(args, "--command", "echo", "Successfully Installed")...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	nix.Lock()
	nix.Packages = append(nix.Packages, pkgs...)
	nix.Unlock()

	return nil
}
