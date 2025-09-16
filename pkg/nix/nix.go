package nix

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
	NixyVersion string `yaml:"-"`

	ConfigFile     *string `yaml:"-"`
	hasHashChanged bool    `yaml:"-"`

	executor Executor `yaml:"-"`

	executorArgs *ExecutorArgs `yaml:"-"`

	sync.Mutex `yaml:"-"`
	Logger     *slog.Logger `yaml:"-"`

	profile *Profile `yaml:"-"`

	cwd string `yaml:"-"`

	NixPkgs   string               `yaml:"nixpkgs"`
	Packages  []*NormalizedPackage `yaml:"packages"`
	Libraries []string             `yaml:"libraries,omitempty"`

	Env map[string]string `yaml:"env,omitempty"`

	ShellHook string `yaml:"shellHook,omitempty"`

	Builds map[string]Build `yaml:"builds,omitempty"`
}

var nixyEnvVars struct {
	NixyProfile    string
	NixyExecutor   Executor
	NixyUseProfile bool
}

func init() {
	if v, ok := os.LookupEnv("NIXY_PROFILE"); ok {
		nixyEnvVars.NixyProfile = v
	} else {
		nixyEnvVars.NixyProfile = "default"
	}

	if v, ok := os.LookupEnv("NIXY_EXECUTOR"); ok {
		nixyEnvVars.NixyExecutor = Executor(v)
	} else {
		nixyEnvVars.NixyExecutor = "local"
	}

	if v, ok := os.LookupEnv("NIXY_USE_PROFILE"); ok {
		v = strings.TrimSpace(v)
		nixyEnvVars.NixyUseProfile = v == "1" || strings.EqualFold(v, "true")
	} else {
		nixyEnvVars.NixyUseProfile = false
	}
}

type ExecutorArgs struct {
	PWD string

	NixBinaryMountedPath  string
	ProfileDirMountedPath string
	FakeHomeMountedPath   string
	NixDirMountedPath     string

	WorkspaceFlakeDirHostPath    string
	WorkspaceFlakeDirMountedPath string

	EnvVars ExecutorEnvVars
}

type Build struct {
	Packages []*NormalizedPackage `yaml:"packages"`
	Paths    []string             `yaml:"paths"`
}

func LoadFromFile(ctx context.Context, f string) (*Nix, error) {
	b, err := os.ReadFile(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read nixy file (%s): %w", f, err)
	}

	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to read current working directory: %w", err)
	}

	profile, err := NewProfile(ctx, nixyEnvVars.NixyProfile)

	nix := Nix{
		profile:  profile,
		executor: nixyEnvVars.NixyExecutor,
		Logger:   slog.Default(),
		cwd:      dir,
	}

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

	hasher := sha256.New()
	hasher.Write([]byte(os.Getenv("NIXY_VERSION")))
	hasher.Write(b)
	sha256Hash := fmt.Sprintf("%x", hasher.Sum(nil))[:7]

	nix.ConfigFile = &f

	if err := os.MkdirAll(nix.executorArgs.WorkspaceFlakeDirHostPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create dir %s: %s", nix.executorArgs.WorkspaceFlakeDirHostPath, err)
	}

	hashFilePath := filepath.Join(nix.executorArgs.WorkspaceFlakeDirHostPath, "nixy.yml.sha256")

	if f == nix.profile.ProfileNixyYAMLPath {
		hashFilePath = filepath.Join(nix.profile.ProfilePath, "nixy.yml.sha256")
	}

	nix.hasHashChanged = true

	if exists(hashFilePath) {
		hash, err := os.ReadFile(hashFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read hash file (%s): %w", hashFilePath, err)
		}

		slog.Debug("comparing nixy.yml hash", "nixy-file", f, "hash file path", hashFilePath, "hash", string(hash), "sha256hash", sha256Hash)
		nix.hasHashChanged = string(hash) != sha256Hash
	}

	if nix.hasHashChanged {
		if err := os.WriteFile(hashFilePath, []byte(sha256Hash), 0o644); err != nil {
			return nil, fmt.Errorf("failed to write sha256 hash (path: %s): %w", hashFilePath, err)
		}
	}

	nixyEnvMap := nix.executorArgs.EnvVars.toMap()

	hasPkgUpdate := false
	for _, pkg := range nix.Packages {
		if pkg == nil {
			continue
		}

		// Fetch SHA256 if not provided
		if pkg.URLPackage != nil {
			pkg.URLPackage.RenderedURL = os.Expand(pkg.URLPackage.URL, func(key string) string {
				if v, ok := nixyEnvMap[key]; ok {
					return v
				}
				return os.Getenv(key)
			})

			_, hasSha256 := pkg.URLPackage.Sha256[getOSArch()]
			if hasSha256 {
				continue
			}

			hash, err := fetchURLHash(pkg.URLPackage.RenderedURL)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch SHA256 hash for (name: %s, url: %s): %w", pkg.URLPackage.Name, pkg.URLPackage.URL, err)
			}
			hasPkgUpdate = true
			if pkg.URLPackage.Sha256 == nil {
				pkg.URLPackage.Sha256 = make(map[string]string, 1)
			}
			pkg.URLPackage.Sha256[getOSArch()] = hash
		}
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
		if pkg == nil {
			continue
		}

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
	profile, err := NewProfile(ctx, nixyEnvVars.NixyProfile)
	if err != nil {
		return err
	}

	n := Nix{ConfigFile: &dest, NixPkgs: profile.NixPkgsCommitHash}
	return n.SyncToDisk()
}
