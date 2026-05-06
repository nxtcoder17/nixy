package nixy

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nxtcoder17/nixy/pkg/nixy/templates"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

// Profile represents profile metadata and runtime state.
type Profile struct {
	Name              string `json:"name"`
	NixPkgsCommitHash string `json:"nixpkgs,omitempty"`
	ProfilePath       string `json:"profilePath,omitempty"` // ~/.local/share/nixy/profiles/<name>
}

type profileMetadata struct {
	Name              string `yaml:"name,omitempty"`
	NixPkgsCommitHash string `yaml:"nixpkgs"`
}

var (
	ProfileSandboxMountPath        = "/profile"
	WorkspaceDirSandboxMountPath   = "/workspace"
	WorkspaceFlakeSandboxMountPath = "/workspace-flake"
)

// GetProfile loads an existing profile from disk
func GetProfile(_ *Context, name string) (*Profile, error) {
	profilePath := filepath.Join(XDGDataDir(), "profiles", name)
	profileYAMLPath := filepath.Join(profilePath, "profile.yaml")

	if exists(profileYAMLPath) {
		b, err := os.ReadFile(profileYAMLPath)
		if err != nil {
			return nil, err
		}

		var metadata profileMetadata
		if err := yaml.Unmarshal(b, &metadata); err != nil {
			var nested struct {
				NixPkgs NixPkgsMap `yaml:"nixpkgs"`
			}
			if nestedErr := yaml.Unmarshal(b, &nested); nestedErr != nil {
				return nil, err
			}
			metadata.NixPkgsCommitHash = nested.NixPkgs["default"]
		}
		p := Profile{
			Name:              name,
			NixPkgsCommitHash: metadata.NixPkgsCommitHash,
			ProfilePath:       profilePath,
		}
		return &p, nil
	}

	profileJSONPath := filepath.Join(profilePath, "profile.json")
	if !exists(profileJSONPath) {
		return nil, fmt.Errorf("profile path does not exist")
	}
	b, err := os.ReadFile(profileJSONPath)
	if err != nil {
		return nil, err
	}

	var p Profile

	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	p.Name = name
	p.ProfilePath = profilePath

	if err := p.Save(); err != nil {
		return nil, err
	}

	return &p, nil
}

// NewProfile creates a new profile metadata instance.
func NewProfile(ctx *Context, name string, runtimePaths *RuntimePaths) (*Profile, error) {
	if v, err := GetProfile(ctx, name); err == nil {
		return v, nil
	}

	profilePath := runtimePaths.BasePath

	var nixPkgsHash string
	if term.IsTerminal(int(os.Stdout.Fd())) {
		var err error
		nixPkgsHash, err = fetchCurrentNixpkgsHash(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get current nixpkgs hash: %w", err)
		}
	}

	p := Profile{
		Name:              name,
		ProfilePath:       profilePath,
		NixPkgsCommitHash: nixPkgsHash,
	}

	if err := p.Save(); err != nil {
		return nil, fmt.Errorf("failed to save profile metadata: %w", err)
	}

	b, err := templates.RenderNixConf()
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(filepath.Join(runtimePaths.FakeHomeDir, ".config", "nix", "nix.conf"), b, 0o644); err != nil {
		return nil, fmt.Errorf("failed to create profile's nix.conf: %w", err)
	}

	return &p, nil
}

// Save persists the profile to disk as YAML metadata.
func (p *Profile) Save() error {
	b, err := yaml.Marshal(profileMetadata{
		Name:              p.Name,
		NixPkgsCommitHash: p.NixPkgsCommitHash,
	})
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(p.ProfilePath, "profile.yaml"), b, 0o644)
}

// ProfileList returns all available profiles
func ProfileList(ctx context.Context) ([]string, error) {
	de, err := os.ReadDir(filepath.Join(XDGDataDir(), "profiles"))
	if err != nil {
		return nil, err
	}

	profiles := make([]string, 0, len(de))

	for i := range de {
		if de[i].Type().IsDir() {
			profiles = append(profiles, de[i].Name())
		}
	}

	return profiles, nil
}

// ProfileCreate creates a new profile with the given name
func ProfileCreate(ctx context.Context, name string) error {
	nixyCtx, err := NewContext(ctx, "")
	if err != nil {
		return err
	}

	runtimePaths, err := NewRuntimePaths(name)
	if err != nil {
		return err
	}

	_, err = NewProfile(nixyCtx, name, runtimePaths)
	return err
}

// ProfileEdit opens the profile metadata in the user's editor
func ProfileEdit(ctx context.Context, name string) error {
	nixyCtx, err := NewContext(ctx, "")
	if err != nil {
		return err
	}

	if name == "" {
		name = nixyCtx.NixyProfile
	}

	runtimePaths, err := NewRuntimePaths(name)
	if err != nil {
		return err
	}

	profile, err := NewProfile(nixyCtx, name, runtimePaths)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, os.Getenv("EDITOR"), "profile.yaml")
	cmd.Dir = profile.ProfilePath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

// fetchCurrentNixpkgsHash fetches the latest nixpkgs commit hash
func fetchCurrentNixpkgsHash(ctx context.Context) (string, error) {
	if !askUser("Fetching Current NixPkgs Version ?") {
		return "", fmt.Errorf("User Aborted fetching current nixpkgs version")
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/nixos/nixpkgs/commits/nixos-unstable", nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return "", err
	}

	var result struct {
		SHA string `json:"sha"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	resp.Body.Close()

	return result.SHA, nil
}

func downloader(msg string, reader io.Reader, writer io.Writer) error {
	totalBytes := 0

	b := make([]byte, 0xffff)
	for {
		n, err := reader.Read(b)
		totalBytes += n
		fmt.Printf("\033[2K\r%s ... %.2fMBs", msg, float64(totalBytes)/1024/1024)
		if _, err := writer.Write(b[:n]); err != nil {
			return fmt.Errorf("failed to write to output path: %w", err)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Printf("\n")
				fmt.Printf("\033[2K\rDownload Finished ... %.2fMBs\n", float64(totalBytes)/1024/1024)
				break
			}
			return err
		}
	}

	return nil
}

// downloadStaticNixBinary downloads the static nix binary for bubblewrap profiles
func downloadStaticNixBinary(ctx context.Context, binPath string) error {
	_, err := os.Stat(binPath)
	if err == nil {
		fmt.Println("PATH already exists")
		return nil
	}

	if !askUser(fmt.Sprintf("Downloading Static Nix Binary to %s ? ", binPath)) {
		return fmt.Errorf("User did not allow downloading static nix binary")
	}

	url := "https://hydra.nixos.org/job/nix/master/buildStatic.nix-cli.x86_64-linux/latest/download/1"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	output, err := os.OpenFile(binPath,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, // flags
		0o755,                              // permissions
	)
	if err != nil {
		return err
	}
	defer output.Close()

	if err := downloader("Downloading Static Nix Executable", resp.Body, output); err != nil {
		return err
	}

	return nil
}

// askUser prompts the user for a yes/no response
func askUser(message string) bool {
	fmt.Printf("%s (Y/N): ", message)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input) // remove newline + spaces

	switch strings.ToLower(input) {
	case "y", "yes":
		return true
	default:
		return false
	}
}
