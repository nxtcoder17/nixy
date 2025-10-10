package nixy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nxtcoder17/nixy/pkg/nixy/templates"
)

// Profile represents a general profile that all executors can use
type Profile struct {
	Name                string
	NixPkgsCommitHash   string `json:"nixpkgs,omitempty"`
	ProfilePath         string // ~/.local/share/nixy/profiles/<name>
	FakeHomeDir         string
	WorkspacesDir       string
	NixDir              string
	StaticNixBinPath    string // Path for static nix binary
	ProfileNixyYAMLPath string
}

var (
	ProfileSandboxMountPath        string = "/profile"
	WorkspaceDirSandboxMountPath   string = "/workspace"
	WorkspaceFlakeSandboxMountPath string = "/workspace-flake"
)

func GetProfile(ctx *Context, name string) (*Profile, error) {
	profileJSONPath := filepath.Join(XDGDataDir(), "profiles", name, "profile.json")

	b, err := os.ReadFile(profileJSONPath)
	if err != nil {
		return nil, err
	}

	var p Profile

	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}

	return &p, nil
}

// NewProfile creates a new profile instance
func NewProfile(ctx *Context, name string) (*Profile, error) {
	if v, err := GetProfile(ctx, name); err == nil {
		return v, nil
	}

	profilePath := filepath.Join(XDGDataDir(), "profiles", name)

	nixpkgsHash, err := fetchCurrentNixpkgsHash(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current nixpkgs hash: %w", err)
	}

	nixDir := filepath.Join(profilePath, "nix")

	fakeHomeDir := filepath.Join(profilePath, "fake-home")

	p := Profile{
		Name:                name,
		ProfilePath:         profilePath,
		NixPkgsCommitHash:   nixpkgsHash,
		WorkspacesDir:       filepath.Join(profilePath, "workspaces"),
		FakeHomeDir:         fakeHomeDir,
		NixDir:              nixDir,
		StaticNixBinPath:    filepath.Join(nixDir, "bin", "nix"),
		ProfileNixyYAMLPath: filepath.Join(profilePath, "nixy.yml"),
	}

	if err := p.CreateDirs(); err != nil {
		return nil, fmt.Errorf("failed to create profile level directories: %w", err)
	}

	if err := p.Save(); err != nil {
		return nil, fmt.Errorf("failed to save profile into a profile.json: %w", err)
	}

	b, err := templates.RenderProfileNixyYAML(templates.ProfileNixyYAMLParams{NixPkgsCommit: nixpkgsHash})
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(p.ProfileNixyYAMLPath, b, 0o644); err != nil {
		return nil, fmt.Errorf("failed to create profile flake.nix: %w", err)
	}

	b, err = templates.RenderNixConf()
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(filepath.Join(p.FakeHomeDir, ".config", "nix", "nix.conf"), b, 0o644); err != nil {
		return nil, fmt.Errorf("failed to create profile's nix.conf: %w", err)
	}

	return &p, nil
}

func (p *Profile) Save() error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(p.ProfilePath, "profile.json"), b, 0o644)
}

// CreateDirs creates the necessary directories for the profile
func (p *Profile) CreateDirs() error {
	dirs := []string{
		p.ProfilePath,
		p.WorkspacesDir,
		p.FakeHomeDir,
		filepath.Dir(p.StaticNixBinPath),

		// we need to have this nix dir to be used for nix store
		filepath.Join(p.NixDir, "var", "nix"),
		filepath.Join(p.FakeHomeDir, ".config", "nix"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return nil
}
