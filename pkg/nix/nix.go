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
	Executor string
	Profile  string
}

type Nix struct {
	ConfigFile *string `json:"-"`

	executor Executor `json:"-"`

	sync.Mutex `json:"-"`
	Logger     *slog.Logger `json:"-"`

	NixPkgs string `json:"nixpkgs"`

	Packages   []string          `json:"packages"`
	bubbleWrap BubbleWrapProfile `json:"-"`
}

func LoadFromFile(ctx Context, f string) (*Nix, error) {
	b, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}

	nix := Nix{bubbleWrap: UseBubbleWrap(ctx.Profile)}

	if err := yaml.Unmarshal(b, &nix); err != nil {
		return nil, err
	}

	nix.ConfigFile = &f

	nix.executor = Executor(ctx.Executor)
	switch nix.executor {
	case BubbleWrapExecutor:
		nix.bubbleWrap.createFSDirs()
	}

	hostPath, _ := nix.FlakeDir()
	if err := os.MkdirAll(hostPath, 0o755); err != nil {
		return nil, err
	}

	return &nix, err
}

func (n *Nix) FlakeDir() (host, mounted string) {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	cwdHash := fmt.Sprintf("%x-%s", md5.New().Sum([]byte(cwd)), filepath.Base(cwd))

	if n.executor == BubbleWrapExecutor {
		return filepath.Join(n.bubbleWrap.WorkspacesDir.HostPath, cwdHash), filepath.Join(n.bubbleWrap.WorkspacesDir.MountedPath, cwdHash)
	}
	// For non-bubblewrap executors, use a temp nixy directory
	return filepath.Join(os.TempDir(), "nixy", cwdHash), filepath.Join(os.TempDir(), "nixy", cwdHash)
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

func InitNixyFile(ctx context.Context, dest string, profileName string, executor string) error {
	n := Nix{ConfigFile: &dest, bubbleWrap: UseBubbleWrap(profileName)}

	nixPkgsCommit, err := func() (string, error) {
		switch Executor(executor) {
		case BubbleWrapExecutor:
			pm, err := n.bubbleWrap.ProfileMetadata()
			if err != nil {
				return "", err
			}
			return pm.NixPkgsCommit, nil
		default:
			return fetchCurrentNixpkgsHash(ctx)
		}
	}()
	if err != nil {
		return err
	}

	n.NixPkgs = nixPkgsCommit
	return n.SyncToDisk()
}
