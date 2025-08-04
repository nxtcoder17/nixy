package nix

import (
	"os"
	"sync"

	"sigs.k8s.io/yaml"
)

type Nix struct {
	ConfigFile *string `json:"-"`

	Packages []string `json:"packages"`

	sync.Mutex
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
		if err := os.WriteFile(*n.ConfigFile, b, 0o66); err != nil {
			return err
		}
	}

	return nil
}
