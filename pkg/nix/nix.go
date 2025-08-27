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

type Nix struct {
	ConfigFile *string `json:"-"`

	executor Executor `json:"-"`

	sync.Mutex `json:"-"`
	Logger     *slog.Logger `json:"-"`

	NixPkgs string `json:"nixpkgs"`

	Packages   []string    `json:"packages"`
	profile    *Profile    `json:"-"`
	bubbleWrap *BubbleWrap `json:"-"`
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

	if err := yaml.Unmarshal(b, &nix); err != nil {
		return nil, err
	}

	nix.ConfigFile = &f

	hostPath, _ := nix.FlakeDir()

	if err := os.MkdirAll(hostPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create dir %s: %s", hostPath, err)
	}

	return &nix, nil
}

func (n *Nix) FlakeDir() (host, mounted string) {
	cwd, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("FAILED to read current working directory: %w", err))
	}

	cwdHash := fmt.Sprintf("%x-%s", md5.Sum([]byte(cwd)), filepath.Base(cwd))

	hostPath := filepath.Join(n.profile.WorkspacesDir, cwdHash)

	if n.executor == BubbleWrapExecutor {
		return hostPath, filepath.Join(n.bubbleWrap.WorkspacesDirMountedPath, cwdHash)
	}
	// For non-bubblewrap executors, use profile workspaces directory
	return hostPath, hostPath
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
	profile, err := NewProfile(ctx, GetCurrentNixyProfile())
	if err != nil {
		return err
	}

	n := Nix{ConfigFile: &dest, NixPkgs: profile.NixPkgsCommitHash}
	return n.SyncToDisk()
}
