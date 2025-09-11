package nix

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/nxtcoder17/nixy/pkg/nix/templates"
	"github.com/nxtcoder17/nixy/pkg/set"
	"gopkg.in/yaml.v3"
)

type NixPackage struct {
	Name   string
	Commit string
}

type URLPackage struct {
	Name   string `yaml:"name"`
	URL    string `yaml:"url"`
	Sha256 string `yaml:"sha256,omitempty"`
}

type NormalizedPackage struct {
	*NixPackage
	*URLPackage
}

func (p *NormalizedPackage) UnmarshalYAML(value *yaml.Node) error {
	// try string form
	var s string
	if err := value.Decode(&s); err == nil {
		np, err := parseNixPackage(s)
		if err != nil {
			return err
		}
		*p = *np
		return nil
	}

	// else try object form
	var urlpkg URLPackage
	if err := value.Decode(&urlpkg); err != nil {
		return err
	}

	if urlpkg.Name == "" {
		return fmt.Errorf("invalid URL package, must specify the name")
	}

	if urlpkg.URL == "" {
		return fmt.Errorf("invalid URL package, must specify the URL")
	}

	*p = NormalizedPackage{
		URLPackage: &urlpkg,
	}
	return nil
}

func (p *NormalizedPackage) MarshalYAML() (any, error) {
	if p.NixPackage != nil {
		if p.NixPackage.Commit == "" {
			return p.NixPackage.Name, nil
		}

		return fmt.Sprintf("nixpkgs/%s#%s", p.NixPackage.Commit, p.NixPackage.Name), nil
	}

	// if only Name, emit as string
	if p.URLPackage != nil {
		return map[string]string{
			"name":   p.URLPackage.Name,
			"url":    p.URLPackage.URL,
			"sha256": p.URLPackage.Sha256,
		}, nil
	}

	return p, nil
}

func parseNixPackage(pkg string) (*NormalizedPackage, error) {
	if !strings.HasPrefix(pkg, "nixpkgs/") {
		return &NormalizedPackage{NixPackage: &NixPackage{Name: pkg, Commit: ""}}, nil
	}
	// Parse nixpkgs/COMMIT#package format
	parts := strings.Split(pkg, "#")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid package format: %s", pkg)
	}

	return &NormalizedPackage{NixPackage: &NixPackage{
		Name:   parts[1],
		Commit: strings.TrimPrefix(parts[0], "nixpkgs/"),
	}}, nil
}

type WorkspaceFlakeGenParams struct {
	NixPkgsDefaultCommit string
	WorkspaceDirPath     string
	Packages             []*NormalizedPackage
	Libraries            []string
	Builds               map[string]Build
}

func genWorkspaceFlakeParams(params WorkspaceFlakeGenParams) (*templates.WorkspaceFlakeParams, error) {
	cache := make(map[string]struct{})

	result := templates.WorkspaceFlakeParams{
		NixPkgsDefaultCommit: params.NixPkgsDefaultCommit,
		NixPkgsCommits:       []string{},
		PackagesMap:          map[string][]string{},
		LibrariesMap:         map[string][]string{},
		URLPackages:          []templates.URLPackage{},
		WorkspaceDir:         params.WorkspaceDirPath,
		Builds:               map[string]templates.WorkspaceFlakePackgeBuild{},
	}

	packagesMap := map[string]*set.Set[string]{}
	librariesMap := map[string]*set.Set[string]{}

	for _, pkg := range params.Packages {
		if pkg == nil {
			continue
		}

		if pkg.NixPackage != nil {
			nixpkg := pkg.NixPackage

			if nixpkg.Commit == "" {
				nixpkg.Commit = params.NixPkgsDefaultCommit
			}

			if _, ok := cache[nixpkg.Commit]; !ok {
				cache[nixpkg.Commit] = struct{}{}
				result.NixPkgsCommits = append(result.NixPkgsCommits, nixpkg.Commit)
			}

			if _, ok := packagesMap[nixpkg.Commit]; !ok {
				packagesMap[nixpkg.Commit] = &set.Set[string]{}
			}
			packagesMap[nixpkg.Commit].Add(nixpkg.Name)
		}

		if pkg.URLPackage != nil {
			result.URLPackages = append(result.URLPackages, templates.URLPackage{
				Name:   pkg.URLPackage.Name,
				URL:    pkg.URLPackage.URL,
				Sha256: pkg.URLPackage.Sha256,
			})
		}
	}

	for _, pkg := range params.Libraries {
		np, err := parseNixPackage(pkg)
		if err != nil {
			return nil, fmt.Errorf("failed to parse library (%s): %w", pkg, err)
		}

		if np.NixPackage == nil {
			return nil, fmt.Errorf("library (%s) must be a nix package", pkg)
		}

		nixpkg := np.NixPackage

		if nixpkg.Commit == "" {
			nixpkg.Commit = params.NixPkgsDefaultCommit
		}

		if _, ok := cache[nixpkg.Commit]; !ok {
			cache[nixpkg.Commit] = struct{}{}
			result.NixPkgsCommits = append(result.NixPkgsCommits, nixpkg.Commit)
		}

		if _, ok := librariesMap[nixpkg.Commit]; !ok {
			librariesMap[nixpkg.Commit] = &set.Set[string]{}
		}
		librariesMap[nixpkg.Commit].Add(nixpkg.Name)
	}

	for key, build := range params.Builds {
		pkgBuild := templates.WorkspaceFlakePackgeBuild{
			PackagesMap: map[string][]string{},
			Paths:       build.Paths,
		}

		for _, pkg := range build.Packages {
			if pkg.NixPackage != nil {
				nixpkg := pkg.NixPackage

				if nixpkg.Commit == "" {
					nixpkg.Commit = params.NixPkgsDefaultCommit
				}

				if _, ok := cache[nixpkg.Commit]; !ok {
					cache[nixpkg.Commit] = struct{}{}
					result.NixPkgsCommits = append(result.NixPkgsCommits, nixpkg.Commit)
				}

				pkgBuild.PackagesMap[nixpkg.Commit] = append(pkgBuild.PackagesMap[nixpkg.Commit], nixpkg.Name)
			}
		}

		result.Builds[key] = pkgBuild
	}

	for k, v := range packagesMap {
		result.PackagesMap[k] = v.ToSortedList()
	}

	for k, v := range librariesMap {
		result.LibrariesMap[k] = v.ToSortedList()
	}

	slices.Sort(result.NixPkgsCommits)
	slices.SortFunc(result.URLPackages, func(a, b templates.URLPackage) int {
		return strings.Compare(a.Name, b.Name)
	})

	return &result, nil
}

// fetchURLHash fetches the SHA256 hash of a file at the given URL
func fetchURLHash(url string) (string, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch URL: status %d", resp.StatusCode)
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, resp.Body); err != nil {
		return "", fmt.Errorf("failed to hash content: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
