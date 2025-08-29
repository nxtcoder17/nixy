package nix

import (
	"context"
	"crypto/md5"
	"fmt"
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

type Context struct {
	Executor Executor
	Profile  string
}

// URLPackage represents a custom package to be fetched from a URL
type URLPackage struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Sha256 string `json:"sha256,omitempty"`
	Type   string `json:"type,omitempty"` // "binary" or "archive", auto-detected if empty
}

type Nix struct {
	ConfigFile *string `json:"-"`

	executor Executor `json:"-"`

	sync.Mutex `json:"-"`
	Logger     *slog.Logger `json:"-"`

	profile    *Profile    `json:"-"`
	bubbleWrap *BubbleWrap `json:"-"`
	docker     *Docker     `json:"-"`

	NixPkgs       string              `json:"nixpkgs"`
	InputPackages []any               `json:"packages"` // Can be string or PackageConfig
	Packages      []NormalizedPackage `json:"-"`
	Libraries     []string            `json:"libraries,omitempty"`

	ShellHook string `json:"shellHook,omitempty"`
}

func GetCurrentNixyProfile() string {
	if v, ok := os.LookupEnv("NIXY_PROFILE"); ok {
		return v
	}
	return "default"
}

func GetCurrentNixyExecutor() Executor {
	if v, ok := os.LookupEnv("NIXY_EXECUTOR"); ok {
		return Executor(v)
	}
	return LocalExecutor
}

func LoadFromFile(ctx context.Context, f string) (*Nix, error) {
	b, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}

	profile, err := NewProfile(ctx, GetCurrentNixyProfile())

	nix := Nix{profile: profile, executor: GetCurrentNixyExecutor(), Logger: slog.Default()}
	if nix.executor == BubbleWrapExecutor {
		bwrap, err := UseBubbleWrap(profile)
		if err != nil {
			return nil, err
		}
		nix.bubbleWrap = bwrap
	}

	if nix.executor == DockerExecutor {
		docker, err := UseDocker(profile)
		if err != nil {
			return nil, err
		}
		nix.docker = docker
	}

	if err := yaml.Unmarshal(b, &nix); err != nil {
		return nil, err
	}

	nix.ConfigFile = &f

	hostPath, _ := nix.WorkspaceFlakeDir()

	if err := os.MkdirAll(hostPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create dir %s: %s", hostPath, err)
	}

	np, err := nix.parseAndUpdatePackageList()
	if err != nil {
		return nil, err
	}

	nix.Packages = np

	return &nix, nil
}

func (n *Nix) WorkspaceFlakeDir() (host, mounted string) {
	cwd, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("FAILED to read current working directory: %w", err))
	}

	cwdHash := fmt.Sprintf("%x-%s", md5.Sum([]byte(cwd)), filepath.Base(cwd))

	hostPath := filepath.Join(n.profile.WorkspacesDir, cwdHash)

	switch n.executor {
	case BubbleWrapExecutor:
		return hostPath, filepath.Join(n.bubbleWrap.WorkspacesDirMountedPath, cwdHash)
	case DockerExecutor:
		return hostPath, filepath.Join(n.docker.WorkspacesDirMountedPath, cwdHash)
	}

	return hostPath, hostPath
}

func (n *Nix) ProfileFlakeDir() (host, mounted string) {
	if n.executor == BubbleWrapExecutor {
		return n.profile.ProfileFlakeDir, n.bubbleWrap.ProfileFlakeDirMountedPath
	}
	return n.profile.ProfileFlakeDir, n.profile.ProfileFlakeDir
}

func (n *Nix) SyncToDisk() error {
	n.Lock()
	defer n.Unlock()

	// Deduplicate packages while preserving any type
	upkg := make([]any, 0, len(n.InputPackages))
	set := make(map[string]struct{}, len(n.InputPackages))

	for _, pkg := range n.InputPackages {
		var key string
		switch v := pkg.(type) {
		case string:
			key = v
		case map[string]any:
			// For URL packages, use name as key
			if name, ok := v["name"].(string); ok {
				key = name
			}
		case URLPackage:
			// For URL packages, use name as key
			key = v.Name
		default:
			fmt.Printf("go type: %T", v)
		}

		if key == "" {
			return fmt.Errorf("[SHOULD NEVER HAPPEN] failed to decide key for keeping packages unique")
		}

		if _, ok := set[key]; ok {
			continue
		}
		set[key] = struct{}{}
		upkg = append(upkg, pkg)
	}

	n.InputPackages = upkg

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
	profile, err := NewProfile(ctx, GetCurrentNixyProfile())
	if err != nil {
		return err
	}

	n := Nix{ConfigFile: &dest, NixPkgs: profile.NixPkgsCommitHash}
	return n.SyncToDisk()
}
