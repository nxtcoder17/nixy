package nix

import (
	"context"
	"os"
	"strings"
)

func (nix *Nix) Install(ctx context.Context, pkgs ...string) error {
	nixShellCommand := []string{
		"nix", "shell", "--log-format", "bar-with-logs",
	}
	nixShellCommand = append(nixShellCommand, nix.Packages...)

	for _, pkg := range pkgs {
		if strings.HasPrefix(pkg, "nixpkgs#") {
		}

		if !strings.HasPrefix(pkg, "nixpkgs/") {
		}
	}

	nixShellCommand = append(nixShellCommand, pkgs...)
	nixShellCommand = append(nixShellCommand, "--command", "echo", "Successfully Downloaded")

	cmd, err := nix.PrepareNixCommand(ctx, nixShellCommand[0], nixShellCommand[1:])
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		nix.Logger.Error("FAILED to install nix packages", "command", cmd.String(), "err", err)
		return err
	}

	nix.Lock()
	nix.Packages = append(nix.Packages, pkgs...)
	nix.Unlock()

	return nil
}
