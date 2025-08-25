package nix

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"sigs.k8s.io/yaml"
)

type Executor string

const (
	LocalExecutor      Executor = "local"
	DockerExecutor     Executor = "docker"
	BubbleWrapExecutor Executor = "bubblewrap"
)

type Nix struct {
	ConfigFile *string `json:"-"`

	Executor Executor `json:"-"`

	sync.Mutex `json:"-"`
	Logger     *slog.Logger `json:"-"`

	NixPkgs string `json:"nixpkgs"`

	Packages   []string `json:"packages"`
	BubbleWrap `json:"-"`
}

type BubbleWrap struct {
	ProfileName  string
	profilePath  string
	userHome     string
	nixStorePath string
}

type ProfileMetadata struct {
	NixPkgsCommit string `json:"nixpkgs"`
}

func (b *BubbleWrap) ProfilePath() string {
	if b.ProfileName == "" {
		if v, ok := os.LookupEnv("NIXY_PROFILE"); ok {
			b.ProfileName = v
		} else {
			b.ProfileName = "default"
		}
	}

	if b.profilePath == "" {
		b.profilePath = filepath.Join(XDGDataDir(), "profiles", b.ProfileName)
	}
	return b.profilePath
}

func (b *BubbleWrap) writeProfileMetadata(p ProfileMetadata) error {
	b2, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(b.ProfilePath(), "metadata.json"), b2, 0o755)
}

func (b *BubbleWrap) ProfileMetadata() (*ProfileMetadata, error) {
	meta, err := os.ReadFile(filepath.Join(b.ProfilePath(), "metadata.json"))
	if err != nil {
		return nil, err
	}

	var metadata ProfileMetadata

	if err := json.Unmarshal(meta, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

func (b *BubbleWrap) UserHome() string {
	if b.userHome == "" {
		b.userHome = filepath.Join(b.ProfilePath(), "fake-home")
	}
	return b.userHome
}

func (b *BubbleWrap) NixDir() string {
	if b.nixStorePath == "" {
		b.nixStorePath = filepath.Join(b.ProfilePath(), "nix")
	}
	return b.nixStorePath
}

func (b *BubbleWrap) ProfileBinPath() string {
	return filepath.Join(b.ProfilePath(), "bin")
}

func (b *BubbleWrap) ProfileSetupDir() string {
	return filepath.Join(b.ProfilePath(), "setup")
}

func (b *BubbleWrap) createFSDirs() error {
	if err := os.MkdirAll(b.UserHome(), 0o755); err != nil {
		return err
	}

	if err := os.MkdirAll(b.NixDir(), 0o755); err != nil {
		return err
	}

	if err := os.MkdirAll(b.ProfileBinPath(), 0o755); err != nil {
		return err
	}

	if err := os.MkdirAll(b.ProfileSetupDir(), 0o755); err != nil {
		return err
	}

	return nil
}

func LoadFromFile(f string) (*Nix, error) {
	b, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}

	nix := Nix{}

	if err := yaml.Unmarshal(b, &nix); err != nil {
		return nil, err
	}

	nix.ConfigFile = &f

	if nix.Executor == "" {
		if v, ok := os.LookupEnv("NIXY_EXECUTOR"); ok {
			nix.Executor = Executor(v)
		}
	}

	switch nix.Executor {
	case "":
		nix.Executor = LocalExecutor
	case BubbleWrapExecutor:
		nix.BubbleWrap.createFSDirs()
	}

	return &nix, err
}

func (n *Nix) SyncToDisk() error {
	n.Lock()
	defer n.Unlock()

	upkg := make([]string, 0, len(n.Packages))

	set := make(map[string]struct{}, len(n.Packages))
	for _, pkg := range n.Packages {
		if _, ok := set[pkg]; ok {
			continue
		}
		set[pkg] = struct{}{}
		upkg = append(upkg, pkg)
	}

	n.Packages = upkg

	b, err := yaml.Marshal(n)
	if err != nil {
		return err
	}

	if n.ConfigFile != nil {
		if err := os.WriteFile(*n.ConfigFile, b, 0o644); err != nil {
			return err
		}
	}

	return nil
}

func InitNixyFile(ctx context.Context, dest string) error {
	n := Nix{ConfigFile: &dest}
	metadata, err := n.ProfileMetadata()
	if err != nil {
		return err
	}

	n.NixPkgs = metadata.NixPkgsCommit
	return n.SyncToDisk()
}
