package nix

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func isNixInstalled() bool {
	cmd := exec.Command("nix", "--version")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func (nix *Nix) localShell(ctx ShellContext, program string) (*exec.Cmd, error) {
	nixBin := "nix"

	hostWorkspacePath, _ := nix.WorkspaceFlakeDir()

	var script []string

	if !isNixInstalled() {
		if err := downloadStaticNixBinary(ctx, nix.profile.StaticNixBinPath); err != nil {
			return nil, fmt.Errorf("failed to download static nix binary: %w", err)
		}
		nixBin = nix.profile.StaticNixBinPath
		script = append(script, fmt.Sprintf("export PATH=%s:$PATH", filepath.Dir(nix.profile.StaticNixBinPath)))
	}

	nixShellArgs := []string{
		"shell",
		"--extra-experimental-features",
		"nix-command flakes",
		fmt.Sprintf("nixpkgs/%s#bash", nix.NixPkgs),
		"--command",
		"bash",
		"-c",
		strings.Join(append(script,
			fmt.Sprintf("cd %s", hostWorkspacePath),
			fmt.Sprintf("nix develop --quiet --quiet --override-input profile-flake %s --command %s", nix.profile.ProfileFlakeDir, program),
		), "\n"),
	}

	return exec.CommandContext(ctx, nixBin, nixShellArgs...), nil
}
