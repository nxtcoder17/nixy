package nix

import (
	"fmt"
	"slices"
	"strings"
)

type ParsedPackages struct {
	CommitsList  []string
	PackagesMap  map[string][]string
	LibrariesMap map[string][]string
}

// getPinnedPackages parses packages and groups them by nixpkgs commit
func (n *Nix) parsePackages() (*ParsedPackages, error) {
	cache := make(map[string]struct{}, len(n.Packages)+len(n.Libraries))

	pp := &ParsedPackages{
		CommitsList:  make([]string, 0, len(n.Packages)+len(n.Libraries)),
		PackagesMap:  make(map[string][]string, len(n.Packages)),
		LibrariesMap: make(map[string][]string, len(n.Libraries)),
	}

	parse := func(items []string, out map[string][]string) error {
		for _, pkg := range items {
			if !strings.HasPrefix(pkg, "nixpkgs/") {
				pkg = fmt.Sprintf("nixpkgs/%s#%s", n.NixPkgs, pkg)
			}

			parts := strings.Split(pkg, "#")
			if len(parts) != 2 {
				return fmt.Errorf("invalid package %s", pkg)
			}

			commit := strings.TrimPrefix(parts[0], "nixpkgs/")

			if _, ok := cache[commit]; !ok {
				cache[commit] = struct{}{}
				pp.CommitsList = append(pp.CommitsList, commit)
			}
			out[commit] = append(out[commit], parts[1])
		}

		return nil
	}

	if err := parse(n.Packages, pp.PackagesMap); err != nil {
		return nil, err
	}

	if err := parse(n.Libraries, pp.LibrariesMap); err != nil {
		return nil, err
	}

	slices.Sort(pp.CommitsList)

	return pp, nil
}
