package nixy

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func UseLocal(ctx *Context, runtimePaths *RuntimePaths) (*ExecutorArgs, error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("nix is not installed on your machine. Please follow docs over `https://nixos.org/download/` to install nix on your machine")
		}
	}

	wsHostPath := runtimePaths.WorkspaceNixyDir

	return &ExecutorArgs{
		NixBinaryMountedPath:         nixPath,
		ProfileDirMountedPath:        runtimePaths.BasePath,
		FakeHomeMountedPath:          runtimePaths.FakeHomeDir,
		NixDirMountedPath:            runtimePaths.NixDir,
		WorkspaceFlakeDirHostPath:    wsHostPath,
		WorkspaceFlakeDirMountedPath: wsHostPath,

		EnvVars: executorEnvVars{
			User:     os.Getenv("USER"),
			Home:     os.Getenv("HOME"),
			Term:     os.Getenv("TERM"),
			TermInfo: os.Getenv("TERMINFO"),

			XDGSessionType: os.Getenv("XDG_SESSION_TYPE"),
			XDGCacheHome:   filepath.Join(runtimePaths.FakeHomeDir, ".cache"),
			XDGDataHome:    filepath.Join(runtimePaths.FakeHomeDir, ".local", "share"),
			// Path: []string{
			// 	filepath.Dir(ctx.NixyBinPath),
			// 	filepath.Dir(nixPath),
			// },
			NixyWorkspaceDir:      ctx.PWD,
			NixyWorkspaceFlakeDir: wsHostPath,
			NixConfDir:            "",
		},
	}, nil
}

func (nixy *NixyWrapper) localShell(ctx *Context, command string, args ...string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s:%s", filepath.Dir(ctx.NixyBinPath), os.Getenv("PATH")))
	return cmd, nil
}
