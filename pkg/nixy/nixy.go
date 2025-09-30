package nixy

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

type Mode string

const (
	LocalExecutor      Mode = "local"
	DockerExecutor     Mode = "docker"
	BubbleWrapExecutor Mode = "bubblewrap"
)

func (m Mode) String() string {
	return string(m)
}

type NixyConfig struct {
	NixPkgs   string               `yaml:"nixpkgs"`
	Packages  []*NormalizedPackage `yaml:"packages"`
	Libraries []string             `yaml:"libraries,omitempty"`

	Env map[string]string `yaml:"env,omitempty"`

	OnShellEnter string `yaml:"onShellEnter,omitempty"`

	// OnShellExit is not used as of now, will try to use it in future
	OnShellExit string `yaml:"onShellExit,omitempty"`

	Builds map[string]Build `yaml:"builds,omitempty"`
}

type Nixy struct {
	Context *Context

	ConfigFile     *string
	hasHashChanged bool
	executorArgs   *ExecutorArgs `yaml:"-"`
	sync.Mutex     `yaml:"-"`
	Logger         *slog.Logger `yaml:"-"`
	profile        *Profile     `yaml:"-"`

	PWD string

	NixyConfig
}

type ExecutorArgs struct {
	NixBinaryMountedPath  string
	ProfileDirMountedPath string
	FakeHomeMountedPath   string
	NixDirMountedPath     string

	WorkspaceFlakeDirHostPath    string
	WorkspaceFlakeDirMountedPath string

	EnvVars executorEnvVars
}

type Build struct {
	Packages []*NormalizedPackage `yaml:"packages"`
	Paths    []string             `yaml:"paths"`
}

type InShellNixy struct {
	PWD    string `yaml:"-"`
	Logger *slog.Logger
	NixyConfig
}

func LoadInNixyShell(parent context.Context) (*InShellNixy, error) {
	workspaceDir, ok := os.LookupEnv("NIXY_WORKSPACE_DIR")
	if !ok {
		return nil, fmt.Errorf("in nixy shell, NIXY_WORKSPACE_DIR must be defined")
	}

	nixyFile := filepath.Join(workspaceDir, "nixy.yml")

	b, err := os.ReadFile(nixyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read nixy file (%s): %w", nixyFile, err)
	}

	nixy := InShellNixy{
		Logger:     slog.Default(),
		PWD:        workspaceDir,
		NixyConfig: NixyConfig{},
	}

	if err := yaml.Unmarshal(b, &nixy.NixyConfig); err != nil {
		return nil, err
	}

	return &nixy, nil
}

func LoadFromFile(parent context.Context, f string) (*Nixy, error) {
	b, err := os.ReadFile(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read nixy file (%s): %w", f, err)
	}

	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to read current working directory: %w", err)
	}

	ctx, err := NewContext(parent, filepath.Dir(f))
	if err != nil {
		return nil, err
	}

	profile, err := NewProfile(ctx, ctx.NixyProfile)

	nixy := Nixy{
		Context: ctx,

		profile:    profile,
		Logger:     slog.Default(),
		PWD:        dir,
		NixyConfig: NixyConfig{},
	}

	switch ctx.NixyMode {
	case BubbleWrapExecutor:
		nixy.executorArgs, err = UseBubbleWrap(ctx, profile)
		if err != nil {
			return nil, err
		}
	case DockerExecutor:
		nixy.executorArgs, err = UseDocker(ctx, profile)
		if err != nil {
			return nil, err
		}
	case LocalExecutor:
		nixy.executorArgs, err = UseLocal(ctx, profile)
		if err != nil {
			return nil, err
		}
	}

	if err := yaml.Unmarshal(b, &nixy.NixyConfig); err != nil {
		return nil, err
	}

	hasher := sha256.New()
	hasher.Write([]byte(os.Getenv("NIXY_VERSION")))
	hasher.Write([]byte(ctx.NixyMode))
	hasher.Write(b)
	sha256Hash := fmt.Sprintf("%x", hasher.Sum(nil))[:7]

	nixy.ConfigFile = &f

	if err := os.MkdirAll(nixy.executorArgs.WorkspaceFlakeDirHostPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create dir %s: %s", nixy.executorArgs.WorkspaceFlakeDirHostPath, err)
	}

	hashFilePath := filepath.Join(nixy.executorArgs.WorkspaceFlakeDirHostPath, "nixy.yml.sha256")

	if f == nixy.profile.ProfileNixyYAMLPath {
		hashFilePath = filepath.Join(nixy.profile.ProfilePath, "nixy.yml.sha256")
	}

	nixy.hasHashChanged = true

	if exists(hashFilePath) {
		hash, err := os.ReadFile(hashFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read hash file (%s): %w", hashFilePath, err)
		}

		slog.Debug("comparing nixy.yml hash", "nixy-file", f, "hash file path", hashFilePath, "hash", string(hash), "sha256hash", sha256Hash)
		nixy.hasHashChanged = string(hash) != sha256Hash
	}

	if nixy.hasHashChanged {
		if err := os.WriteFile(hashFilePath, []byte(sha256Hash), 0o644); err != nil {
			return nil, fmt.Errorf("failed to write sha256 hash (path: %s): %w", hashFilePath, err)
		}
	}

	nixyEnvMap := nixy.executorArgs.EnvVars.toMap(ctx)

	hasPkgUpdate := false
	for _, pkg := range nixy.Packages {
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
		if err := nixy.SyncToDisk(); err != nil {
			return nil, err
		}
	}

	return &nixy, nil
}

func (nixy *Nixy) SyncToDisk() error {
	nixy.Lock()
	defer nixy.Unlock()

	// Deduplicate packages while preserving any type
	upkg := make([]*NormalizedPackage, 0, len(nixy.Packages))
	set := make(map[string]struct{}, len(nixy.Packages))

	for _, pkg := range nixy.Packages {
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

	nixy.Packages = upkg

	b, err := yaml.Marshal(nixy.NixyConfig)
	if err != nil {
		return err
	}

	if nixy.ConfigFile != nil {
		if err := os.WriteFile(*nixy.ConfigFile, b, 0o644); err != nil {
			return err
		}
	}

	return nil
}

func InitNixyFile(parent context.Context, dest string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	ctx, err := NewContext(parent, dir)
	if err != nil {
		return err
	}

	profile, err := NewProfile(ctx, ctx.NixyProfile)
	if err != nil {
		return err
	}

	n := Nixy{
		ConfigFile: &dest,
		NixyConfig: NixyConfig{
			NixPkgs: profile.NixPkgsCommitHash,
		},
	}
	return n.SyncToDisk()
}
