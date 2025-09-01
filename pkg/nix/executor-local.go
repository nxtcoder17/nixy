package nix

import (
	"crypto/md5"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func isNixInstalled() bool {
	cmd := exec.Command("nix", "--version")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func UseLocal(profile *Profile) (*ExecutorArgs, error) {
	if !isNixInstalled() {
		return nil, fmt.Errorf("nix is not installed on your machine")
	}

	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	cwdHash := fmt.Sprintf("%x-%s", md5.Sum([]byte(dir)), filepath.Base(dir))

	wsHostPath := filepath.Join(profile.WorkspacesDir, cwdHash)

	return &ExecutorArgs{
		PWD:                        dir,
		NixBinaryMountedPath:       "nix",
		ProfileFlakeDirMountedPath: profile.ProfileFlakeDir,
		FakeHomeMountedPath:        profile.FakeHomeDir,
		NixDirMountedPath:          profile.NixDir,
		WorkspaceDirHostPath:       wsHostPath,
		WorkspaceDirMountedPath:    wsHostPath,
	}, nil
}

func (nix *Nix) localShell(ctx ShellContext) (func(cmd string, args ...string) *exec.Cmd, error) {
	// nixShellArgs := []string{
	// 	"shell",
	// 	fmt.Sprintf("nixpkgs/%s#bash", nix.NixPkgs),
	// 	"--command",
	// 	"bash",
	// 	"-c",
	// 	strings.Join(append(script,
	// 		fmt.Sprintf("cd %s", hostWorkspacePath),
	// 		fmt.Sprintf("nix develop --quiet --quiet --override-input profile-flake %s --command %s", nix.profile.ProfileFlakeDir, program),
	// 	), "\n"),
	// }

	return func(cmd string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, cmd, args...)
	}, nil
}
