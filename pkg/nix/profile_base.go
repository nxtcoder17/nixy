package nix

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// Profile represents a general profile that all executors can use
type Profile struct {
	Name              string
	NixPkgsCommitHash string `json:"nixpkgs,omitempty"`
	ProfilePath       string // ~/.local/share/nixy/profiles/<name>
	ProfileFlakeDir   string
	FakeHomeDir       string
	WorkspacesDir     string
	NixDir            string
	StaticNixBinPath  string // Path for static nix binary
}

func GetProfile(name string) (*Profile, error) {
	b, err := os.ReadFile(filepath.Join(XDGDataDir(), "profiles", name, "profile.json"))
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
func NewProfile(ctx context.Context, name string) (*Profile, error) {
	if v, err := GetProfile(name); err == nil {
		return v, nil
	}

	profilePath := filepath.Join(XDGDataDir(), "profiles", name)

	nixpkgsHash, err := fetchCurrentNixpkgsHash(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current nixpkgs hash: %w", err)
	}

	nixDir := filepath.Join(profilePath, "nix")

	p := Profile{
		Name:              name,
		ProfilePath:       profilePath,
		NixPkgsCommitHash: nixpkgsHash,
		ProfileFlakeDir:   filepath.Join(profilePath, "profile-flake"),
		WorkspacesDir:     filepath.Join(profilePath, "workspaces"),
		FakeHomeDir:       filepath.Join(profilePath, "fake-home"),
		NixDir:            nixDir,
		StaticNixBinPath:  filepath.Join(nixDir, "bin", "nix"),
	}

	if err := p.CreateDirs(); err != nil {
		return nil, fmt.Errorf("failed to create profile level directories: %w", err)
	}

	if err := p.Save(); err != nil {
		return nil, fmt.Errorf("failed to save profile into a profile.json: %w", err)
	}

	// Create profile flake
	f, err := os.Create(filepath.Join(p.ProfileFlakeDir, "flake.nix"))
	if err != nil {
		return nil, fmt.Errorf("failed to create profile flake.nix: %w", err)
	}

	t := template.New("profile-flake")
	if _, err := t.Parse(templateProfileFlake); err != nil {
		return nil, fmt.Errorf("failed to parse profile flake template: %w", err)
	}
	if err := t.ExecuteTemplate(f, "profile-flake", map[string]any{
		"nixpkgsCommit": p.NixPkgsCommitHash,
	}); err != nil {
		return nil, fmt.Errorf("failed to execute profile flake.nix template: %w", err)
	}
	f.Close()

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
		p.ProfileFlakeDir,
		p.WorkspacesDir,
		p.FakeHomeDir,
		filepath.Dir(p.StaticNixBinPath),

		// we need to have this nix dir to be used for nix store
		filepath.Join(p.NixDir, "var", "nix"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return nil
}
