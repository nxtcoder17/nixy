package nix

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type NixPackage struct {
	Name   string
	Commit string
}

type URLPackage struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Sha256 string `json:"sha256,omitempty"`
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

func (n *Nix) GenerateWorkspaceFlakeParams() (*WorkspaceFlakeParams, error) {
	cache := make(map[string]struct{})

	workspaceDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	result := WorkspaceFlakeParams{
		NixPkgsDefaultCommit: n.NixPkgs,
		NixPkgsCommits:       []string{},
		PackagesMap:          map[string][]string{},
		LibrariesMap:         map[string][]string{},
		URLPackages:          []URLPackage{},
		ProfileFlakeDir:      "",
		WorkspaceDir:         workspaceDir,
	}

	for _, pkg := range n.Packages {
		if pkg.NixPackage != nil {
			nixpkg := pkg.NixPackage

			if nixpkg.Commit == "" {
				nixpkg.Commit = n.NixPkgs
			}

			if _, ok := cache[nixpkg.Commit]; !ok {
				cache[nixpkg.Commit] = struct{}{}
				result.NixPkgsCommits = append(result.NixPkgsCommits, nixpkg.Commit)
			}

			result.PackagesMap[nixpkg.Commit] = append(result.PackagesMap[nixpkg.Commit], nixpkg.Name)
		}

		if pkg.URLPackage != nil {
			result.URLPackages = append(result.URLPackages, *pkg.URLPackage)
		}
	}

	for _, pkg := range n.Libraries {
		np, err := parseNixPackage(pkg)
		if err != nil {
			return nil, fmt.Errorf("failed to parse library (%s): %w", pkg, err)
		}

		if np.NixPackage == nil {
			return nil, fmt.Errorf("library (%s) must be a nix package", pkg)
		}

		nixpkg := np.NixPackage

		if nixpkg.Commit == "" {
			nixpkg.Commit = n.NixPkgs
		}

		if _, ok := cache[nixpkg.Commit]; !ok {
			cache[nixpkg.Commit] = struct{}{}
			result.NixPkgsCommits = append(result.NixPkgsCommits, nixpkg.Commit)
		}

		result.LibrariesMap[nixpkg.Commit] = append(result.LibrariesMap[nixpkg.Commit], nixpkg.Name)
	}

	switch n.executor {
	case LocalExecutor:
		result.ProfileFlakeDir = n.profile.ProfileFlakeDir
	case BubbleWrapExecutor:
		result.ProfileFlakeDir = n.bubbleWrap.ProfileFlakeDirMountedPath
	case DockerExecutor:
		result.ProfileFlakeDir = n.docker.ProfileFlakeDirMountedPath
	}

	slices.Sort(result.NixPkgsCommits)
	slices.SortFunc(result.URLPackages, func(a, b URLPackage) int {
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
