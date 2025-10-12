package nixy

import (
	"crypto/md5"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func deriveWorkspacePath(workspacesDir, cwd string) string {
	sum := md5.Sum([]byte(cwd))
	return filepath.Join(workspacesDir, fmt.Sprintf("%x-%s", sum, filepath.Base(cwd)))
}

func UseLocal(ctx *Context, profile *Profile) (*ExecutorArgs, error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("nix is not installed on your machine. Please follow docs over `https://nixos.org/download/` to install nix on your machine")
		}
	}

	wsHostPath := deriveWorkspacePath(profile.WorkspacesDir, ctx.PWD)

	return &ExecutorArgs{
		NixBinaryMountedPath:         nixPath,
		ProfileDirMountedPath:        profile.ProfilePath,
		FakeHomeMountedPath:          profile.FakeHomeDir,
		NixDirMountedPath:            profile.NixDir,
		WorkspaceFlakeDirHostPath:    wsHostPath,
		WorkspaceFlakeDirMountedPath: wsHostPath,

		EnvVars: executorEnvVars{
			User:     os.Getenv("USER"),
			Home:     os.Getenv("HOME"),
			Term:     os.Getenv("TERM"),
			TermInfo: os.Getenv("TERMINFO"),

			XDGSessionType: os.Getenv("XDG_SESSION_TYPE"),
			XDGCacheHome:   filepath.Join(profile.FakeHomeDir, ".cache"),
			XDGDataHome:    filepath.Join(profile.FakeHomeDir, ".local", "share"),
			Path: []string{
				filepath.Dir(ctx.NixyBinPath),
				filepath.Dir(nixPath),
			},
			NixyWorkspaceDir:      ctx.PWD,
			NixyWorkspaceFlakeDir: wsHostPath,
			NixConfDir:            "",
		},
	}, nil
}

func (nix *Nixy) localShell(ctx *Context, command string, args ...string) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, command, args...), nil
}
