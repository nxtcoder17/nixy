package nixy2

import (
	"github.com/nxtcoder17/go.errors"
	"github.com/nxtcoder17/nixy/internal/app"
	"io/fs"
	"os"
	"path/filepath"
)

func CreateFSPaths(appCtx *app.Context) error {
	// /nix/var/nix is path for a directory to be used for nix store
	nixDir := filepath.Join(appCtx.NixyGlobalDir, "nix")
	fakeHomeDir := filepath.Join(appCtx.NixyGlobalDir, "home")
	artifactDir := filepath.Join(appCtx.PWD, appCtx.NixyProjectDir)

	return createDirs(
		nixDir,
		fakeHomeDir,
		filepath.Join(fakeHomeDir, ".config", "nix"),
		artifactDir,
	)
}

func createDirs(dirs ...string) error {
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return nil
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	if err == nil {
		return true
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	return false
}
