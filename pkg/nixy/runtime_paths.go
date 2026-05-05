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
	WorkspaceNixyDir string // project-local .nixy directory, when a workspace is available
	FakeHomeDir      string // fake home directory for sandboxing
	NixDir           string // nix store directory
	StaticNixBinPath string // path to static nix binary
}

// NewRuntimePaths creates and initializes the runtime paths for a given profile name.
// This is always called regardless of NIXY_USE_PROFILE setting.
func NewRuntimePaths(name string, workspaceDir ...string) (*RuntimePaths, error) {
	rp := runtimePaths(name, workspaceDir...)
	if err := rp.CreateDirs(); err != nil {
		return nil, fmt.Errorf("failed to create runtime directories: %w", err)
	}

	return rp, nil
}

func runtimePaths(name string, workspaceDir ...string) *RuntimePaths {
	basePath := filepath.Join(XDGDataDir(), "profiles", name)
	nixDir := filepath.Join(basePath, "nix")
	fakeHomeDir := filepath.Join(basePath, "fake-home")
	workspaceNixyDir := ""
	if len(workspaceDir) > 0 && workspaceDir[0] != "" {
		workspaceNixyDir = workspaceNixyDirPath(workspaceDir[0])
	}

	return &RuntimePaths{
		Name:             name,
		BasePath:         basePath,
		WorkspaceNixyDir: workspaceNixyDir,
		FakeHomeDir:      fakeHomeDir,
		NixDir:           nixDir,
		StaticNixBinPath: filepath.Join(nixDir, "bin", "nix"),
	}
}

// CreateDirs creates all necessary directories for the runtime paths
func (rp *RuntimePaths) CreateDirs() error {
	for _, dir := range rp.Dirs() {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return nil
}

func (rp *RuntimePaths) Dirs() []string {
	dirs := []string{
		rp.BasePath,
		rp.WorkspaceNixyDir,
		rp.FakeHomeDir,
		filepath.Dir(rp.StaticNixBinPath),
		// we need to have this nix dir to be used for nix store
		filepath.Join(rp.NixDir, "var", "nix"),
		filepath.Join(rp.FakeHomeDir, ".config", "nix"),
	}

	result := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if dir != "" {
			result = append(result, dir)
		}
	}
	return result
}
