package nix

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
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

type Nix struct {
	ConfigFile *string `yaml:"-"`

	executor Executor `yaml:"-"`

	executorArgs *ExecutorArgs `yaml:"-"`

	sync.Mutex `yaml:"-"`
	Logger     *slog.Logger `yaml:"-"`

	profile *Profile `yaml:"-"`

	cwd string `yaml:"-"`

	NixPkgs   string               `yaml:"nixpkgs"`
	Packages  []*NormalizedPackage `yaml:"packages"`
	Libraries []string             `yaml:"libraries,omitempty"`

	ShellHook string `yaml:"shellHook,omitempty"`

	Builds map[string]Build `yaml:"builds,omitempty"`
}

type ExecutorArgs struct {
	PWD string

	NixBinaryMountedPath       string
	ProfileFlakeDirMountedPath string
	FakeHomeMountedPath        string
	NixDirMountedPath          string

	WorkspaceDirHostPath    string
	WorkspaceDirMountedPath string
}

type Build struct {
	Packages []*NormalizedPackage `yaml:"packages"`
	Paths    []string             `yaml:"paths"`
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

	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to read current working directory: %w", err)
	}

	profile, err := NewProfile(ctx, GetCurrentNixyProfile())

	nix := Nix{profile: profile, executor: GetCurrentNixyExecutor(), Logger: slog.Default(), cwd: dir}

	switch nix.executor {
	case BubbleWrapExecutor:
		nix.executorArgs, err = UseBubbleWrap(profile)
		if err != nil {
			return nil, err
		}
	case DockerExecutor:
		nix.executorArgs, err = UseDocker(profile)
		if err != nil {
			return nil, err
		}
	case LocalExecutor:
		nix.executorArgs, err = UseLocal(profile)
		if err != nil {
			return nil, err
		}
	}

	if err := yaml.Unmarshal(b, &nix); err != nil {
		return nil, err
	}

	nix.ConfigFile = &f

	if err := os.MkdirAll(nix.executorArgs.WorkspaceDirHostPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create dir %s: %s", nix.executorArgs.WorkspaceDirHostPath, err)
	}

	hasPkgUpdate := false
	for _, pkg := range nix.Packages {
		// Fetch SHA256 if not provided
		if pkg.URLPackage != nil && pkg.URLPackage.Sha256 == "" {
			hash, err := fetchURLHash(pkg.URLPackage.URL)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch SHA256 hash for (name: %s, url: %s): %w", pkg.URLPackage.Name, pkg.URLPackage.URL, err)
			}
			hasPkgUpdate = true
			pkg.URLPackage.Sha256 = hash
		}

		nix.Packages = append(nix.Packages, pkg)
	}

	if hasPkgUpdate {
		if err := nix.SyncToDisk(); err != nil {
			return nil, err
		}
	}

	return &nix, nil
}

func (n *Nix) SyncToDisk() error {
	n.Lock()
	defer n.Unlock()

	// Deduplicate packages while preserving any type
	upkg := make([]*NormalizedPackage, 0, len(n.Packages))
	set := make(map[string]struct{}, len(n.Packages))

	for _, pkg := range n.Packages {
		var key string

		if pkg.NixPackage != nil {
			key = pkg.NixPackage.Name
		}

		if pkg.URLPackage != nil {
			key = pkg.URLPackage.Name
		}

		if _, ok := set[key]; ok {
			continue
		}
		set[key] = struct{}{}
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
	profile, err := NewProfile(ctx, GetCurrentNixyProfile())
	if err != nil {
		return err
	}

	n := Nix{ConfigFile: &dest, NixPkgs: profile.NixPkgsCommitHash}
	return n.SyncToDisk()
}
