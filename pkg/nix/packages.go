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
	URLPackages  []*URLPackage
}

// parsePackages parses packages and groups them by nixpkgs commit
func (n *Nix) parsePackages() (*ParsedPackages, error) {
	cache := make(map[string]struct{})

	pp := &ParsedPackages{
		CommitsList:  make([]string, 0),
		PackagesMap:  make(map[string][]string),
		LibrariesMap: make(map[string][]string),
		URLPackages:  make([]*URLPackage, 0),
	}

	// Parse mixed package format
	nixPackages, err := n.getNixPackages()
	if err != nil {
		return nil, err
	}
	
	pp.PackagesMap = nixPackages
	for commit := range nixPackages {
		if _, ok := cache[commit]; !ok {
			cache[commit] = struct{}{}
			pp.CommitsList = append(pp.CommitsList, commit)
		}
	}
	
	// Get URL packages
	urlPackages, err := n.getURLPackages()
	if err != nil {
		return nil, err
	}
	pp.URLPackages = urlPackages

	// Parse libraries (still string only)
	for _, lib := range n.Libraries {
		pkg := lib
		if !strings.HasPrefix(pkg, "nixpkgs/") {
			pkg = fmt.Sprintf("nixpkgs/%s#%s", n.NixPkgs, pkg)
		}

		parts := strings.Split(pkg, "#")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid library %s", pkg)
		}

		commit := strings.TrimPrefix(parts[0], "nixpkgs/")

		if _, ok := cache[commit]; !ok {
			cache[commit] = struct{}{}
			pp.CommitsList = append(pp.CommitsList, commit)
		}
		pp.LibrariesMap[commit] = append(pp.LibrariesMap[commit], parts[1])
	}

	slices.Sort(pp.CommitsList)

	return pp, nil
}
