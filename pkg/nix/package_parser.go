package nix

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// NormalizedPackage represents a package in normalized form
type NormalizedPackage struct {
	// For nixpkgs packages
	IsNixPackage bool
	Name         string
	Commit       string // Only for nix packages

	// For URL packages
	IsURLPackage bool
	URLConfig    *URLPackage
}

// parsePackageList normalizes the mixed package format into a consistent structure
func (n *Nix) parseAndUpdatePackageList() ([]NormalizedPackage, error) {
	result := make([]NormalizedPackage, 0, len(n.InputPackages))

	hasUpdate := false

	for i, pkg := range n.InputPackages {
		switch v := pkg.(type) {
		case string:
			// Simple string package from nixpkgs
			np := NormalizedPackage{IsNixPackage: true}

			if !strings.HasPrefix(v, "nixpkgs/") {
				// Use default nixpkgs commit
				np.Name = v
				np.Commit = n.NixPkgs
			} else {
				// Parse nixpkgs/COMMIT#package format
				parts := strings.Split(v, "#")
				if len(parts) != 2 {
					return nil, fmt.Errorf("invalid package format: %s", v)
				}
				np.Commit = strings.TrimPrefix(parts[0], "nixpkgs/")
				np.Name = parts[1]
			}
			result = append(result, np)

		case map[string]any:
			// URL package config
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("failed to parse package config: %w", err)
			}

			var config URLPackage
			if err := json.Unmarshal(jsonBytes, &config); err != nil {
				return nil, fmt.Errorf("invalid package config: %w", err)
			}

			// Fetch SHA256 if not provided
			if config.Sha256 == "" {
				hash, err := fetchURLHash(config.URL)
				if err != nil {
					// Log warning but don't fail - let nix show the error
					fmt.Fprintf(os.Stderr, "Warning: Failed to fetch SHA256 for %s: %v\n", config.URL, err)
				} else {
					config.Sha256 = hash
					n.InputPackages[i] = config
					hasUpdate = true
				}
			}

			result = append(result, NormalizedPackage{
				IsURLPackage: true,
				URLConfig:    &config,
			})

		default:
			return nil, fmt.Errorf("invalid package type (%T): must be string or object", v)
		}
	}

	if hasUpdate {
		if err := n.SyncToDisk(); err != nil {
			return nil, fmt.Errorf("failed to sync nixy.yml to disk: %w", err)
		}
	}

	return result, nil
}

// getURLPackages returns only the URL packages
func (n *Nix) getURLPackages() ([]*URLPackage, error) {
	var urlPackages []*URLPackage
	for _, np := range n.Packages {
		if np.IsURLPackage {
			urlPackages = append(urlPackages, np.URLConfig)
		}
	}

	return urlPackages, nil
}

// getNixPackages returns only the nixpkgs packages grouped by commit
func (n *Nix) getNixPackages() (map[string][]string, error) {
	result := make(map[string][]string)
	for _, np := range n.Packages {
		if np.IsNixPackage {
			result[np.Commit] = append(result[np.Commit], np.Name)
		}
	}

	return result, nil
}
