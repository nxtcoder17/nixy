package nix

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
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
	// For nixpkgs packages
	IsNixPackage bool
	NixPackage   *NixPackage

	// For URL packages
	IsURLPackage bool
	URLConfig    *URLPackage
}

func parsePackage(pkg any) (*NormalizedPackage, error) {
	switch v := pkg.(type) {

	// Simple string package from nixpkgs
	case string:
		if !strings.HasPrefix(v, "nixpkgs/") {
			return &NormalizedPackage{IsNixPackage: true, NixPackage: &NixPackage{Name: v, Commit: ""}}, nil
		}
		// Parse nixpkgs/COMMIT#package format
		parts := strings.Split(v, "#")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid package format: %s", v)
		}

		return &NormalizedPackage{IsNixPackage: true, NixPackage: &NixPackage{
			Name: parts[1], Commit: strings.TrimPrefix(parts[0], "nixpkgs/"),
		}}, nil

	case map[string]any:
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to parse package config: %w", err)
		}

		var urlpkg URLPackage
		if err := json.Unmarshal(jsonBytes, &urlpkg); err != nil {
			return nil, fmt.Errorf("invalid package config: %w", err)
		}

		return &NormalizedPackage{IsURLPackage: true, URLConfig: &urlpkg}, nil
	default:
		return nil, fmt.Errorf("invalid package type (%T): must be string or object", v)
	}
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
		if pkg.IsNixPackage {
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

		if pkg.IsURLPackage {
			result.URLPackages = append(result.URLPackages, *pkg.URLConfig)
		}
	}

	for _, pkg := range n.Libraries {
		np, err := parsePackage(pkg)
		if err != nil {
			return nil, fmt.Errorf("failed to parse library (%s): %w", pkg, err)
		}

		if !np.IsNixPackage {
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

	slog.Info("NixPkgs", "commits", result.NixPkgsCommits)

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
