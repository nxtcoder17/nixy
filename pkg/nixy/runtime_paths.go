package nixy

import (
	"fmt"
	"os"
	"path/filepath"
)

// RuntimePaths represents the filesystem paths needed for nixy runtime execution.
// These paths are always created and used regardless of NIXY_USE_PROFILE setting.
type RuntimePaths struct {
	Name             string // profile name (used for directory organization)
	BasePath         string // ~/.local/share/nixy/profiles/<name>
	WorkspacesDir    string // directory for workspace flakes
	FakeHomeDir      string // fake home directory for sandboxing
	NixDir           string // nix store directory
	StaticNixBinPath string // path to static nix binary
}

// NewRuntimePaths creates and initializes the runtime paths for a given profile name.
// This is always called regardless of NIXY_USE_PROFILE setting.
func NewRuntimePaths(name string) (*RuntimePaths, error) {
	basePath := filepath.Join(XDGDataDir(), "profiles", name)
	nixDir := filepath.Join(basePath, "nix")
	fakeHomeDir := filepath.Join(basePath, "fake-home")

	rp := &RuntimePaths{
		Name:             name,
		BasePath:         basePath,
		WorkspacesDir:    filepath.Join(basePath, "workspaces"),
		FakeHomeDir:      fakeHomeDir,
		NixDir:           nixDir,
		StaticNixBinPath: filepath.Join(nixDir, "bin", "nix"),
	}

	if err := rp.CreateDirs(); err != nil {
		return nil, fmt.Errorf("failed to create runtime directories: %w", err)
	}

	return rp, nil
}

// CreateDirs creates all necessary directories for the runtime paths
func (rp *RuntimePaths) CreateDirs() error {
	dirs := []string{
		rp.BasePath,
		rp.WorkspacesDir,
		rp.FakeHomeDir,
		filepath.Dir(rp.StaticNixBinPath),
		// we need to have this nix dir to be used for nix store
		filepath.Join(rp.NixDir, "var", "nix"),
		filepath.Join(rp.FakeHomeDir, ".config", "nix"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return nil
}
