package nixy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/nxtcoder17/nixy/pkg/nixy/templates"
	"github.com/nxtcoder17/nixy/pkg/set"
	"gopkg.in/yaml.v3"
)

type NixPackage struct {
	Name   string
	Commit string
}

func getOSArch() string {
	return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
}

type URLAndSHA struct {
	URL    string `yaml:"url"`
	SHA256 string `yaml:"sha256"`
}

type URLPackage struct {
	Name        string               `yaml:"name"`
	Sources     map[string]URLAndSHA `yaml:"sources"`
	InstallHook string               `yaml:"installHook,omitempty"`
	BinPaths    []string             `yaml:"binPaths,omitempty"`
}

type NormalizedPackage struct {
	*NixPackage
	*URLPackage
}

// literalString is a helper type that marshals as a YAML literal-style string
type literalString string

func (s literalString) MarshalYAML() (any, error) {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: string(s),
		Style: yaml.LiteralStyle,
	}, nil
}

// urlPackageYAML is the output shape for URLPackage with desired key order
type urlPackageYAML struct {
	Name        string                      `yaml:"name"`
	Sources     orderedSources              `yaml:"sources"`
	BinPaths    []string                    `yaml:"binPaths,omitempty"`
	InstallHook literalString               `yaml:"installHook,omitempty"`
}

// urlSourceYAML ensures url comes before sha256
type urlSourceYAML struct {
	URL    string `yaml:"url"`
	SHA256 string `yaml:"sha256"`
}

// orderedSources wraps the sources map to emit platforms in sorted order
type orderedSources map[string]urlSourceYAML

func (o orderedSources) MarshalYAML() (any, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}

	platforms := make([]string, 0, len(o))
	for p := range o {
		platforms = append(platforms, p)
	}
	slices.Sort(platforms)

	for _, platform := range platforms {
		source := o[platform]
		platformNode := &yaml.Node{Kind: yaml.MappingNode}
		platformNode.Content = append(platformNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "url"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: source.URL},
			&yaml.Node{Kind: yaml.ScalarNode, Value: "sha256"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: source.SHA256},
		)
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: platform},
			platformNode,
		)
	}

	return node, nil
}

func (p *NormalizedPackage) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err == nil {
		np, err := parseNixPackage(s)
		if err != nil {
			return err
		}
		*p = *np
		return nil
	}

	var urlpkg URLPackage
	if err := value.Decode(&urlpkg); err != nil {
		return err
	}

	if urlpkg.Name == "" {
		return fmt.Errorf("invalid URL package, must specify .name")
	}

	if urlpkg.Sources == nil {
		return fmt.Errorf("invalid URL package, must specify .sources")
	}

	for k, v := range urlpkg.Sources {
		if v.URL == "" {
			delete(urlpkg.Sources, k)
		}
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

	if p.URLPackage != nil {
		sources := make(orderedSources, len(p.URLPackage.Sources))
		for platform, source := range p.URLPackage.Sources {
			sources[platform] = urlSourceYAML{URL: source.URL, SHA256: source.SHA256}
		}

		out := urlPackageYAML{
			Name:     p.URLPackage.Name,
			Sources:  sources,
			BinPaths: p.URLPackage.BinPaths,
		}
		if p.URLPackage.InstallHook != "" {
			out.InstallHook = literalString(p.URLPackage.InstallHook)
		}
		return out, nil
	}

	return p, nil
}

func parseNixPackage(pkg string) (*NormalizedPackage, error) {
	parts := strings.SplitN(pkg, "#", 2)

	switch len(parts) {
	case 1:
		// INFO: means just package name
		return &NormalizedPackage{NixPackage: &NixPackage{Name: pkg, Commit: ""}}, nil
	case 2:
		return &NormalizedPackage{NixPackage: &NixPackage{Name: parts[1], Commit: parts[0]}}, nil
	default:
		return nil, fmt.Errorf("invalid package format: %s", pkg)
	}
}

type WorkspaceFlakeGenParams struct {
	NixPkgs          NixPkgsMap
	WorkspaceDirPath string
	Packages         []*NormalizedPackage
	Libraries        []string
	Builds           map[string]Build
	EnvVars          map[string]string
}

func genWorkspaceFlakeParams(params WorkspaceFlakeGenParams) (*templates.WorkspaceFlakeParams, error) {
	result := templates.WorkspaceFlakeParams{
		NixPkgsCommitsList: params.NixPkgs.List(),
		NixPkgsCommitsMap:  params.NixPkgs,
		PackagesMap:        map[string][]string{},
		LibrariesMap:       map[string][]string{},
		URLPackages:        []templates.URLPackage{},
		WorkspaceDir:       params.WorkspaceDirPath,
		Builds:             map[string]templates.WorkspaceFlakePackgeBuild{},
		OSArch:             getOSArch(),
		EnvVars:            params.EnvVars,
	}

	packagesMap := map[string]*set.Set[string]{}
	librariesMap := map[string]*set.Set[string]{}

	for k := range params.NixPkgs {
		packagesMap[k] = &set.Set[string]{}
		librariesMap[k] = &set.Set[string]{}
	}

	for _, pkg := range params.Packages {
		if pkg == nil {
			continue
		}

		if pkg.NixPackage != nil {
			nixpkg := pkg.NixPackage

			if nixpkg.Commit == "" {
				nixpkg.Commit = params.NixPkgs.DefaultCommit()
			}

			packagesMap[nixpkg.Commit].Add(nixpkg.Name)
		}

		if pkg.URLPackage != nil {
			source, ok := pkg.URLPackage.Sources[result.OSArch]
			if !ok || source.URL == "" {
				return nil, fmt.Errorf("URL package %q has no source defined for %s", pkg.URLPackage.Name, result.OSArch)
			}

			result.URLPackages = append(result.URLPackages, templates.URLPackage{
				Name:        pkg.URLPackage.Name,
				URL:         source.URL,
				Sha256:      source.SHA256,
				InstallHook: pkg.URLPackage.InstallHook,
				BinPaths:    pkg.URLPackage.BinPaths,
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
			nixpkg.Commit = params.NixPkgs.DefaultCommit()
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
					nixpkg.Commit = params.NixPkgs.DefaultCommit()
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

	slices.SortFunc(result.URLPackages, func(a, b templates.URLPackage) int {
		return strings.Compare(a.Name, b.Name)
	})

	return &result, nil
}

// fetchURLPackageHash fetches the SHA256 hash of a file at the given URL
func fetchURLPackageHash(url string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("empty URL provided")
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
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

	if err := downloader(fmt.Sprintf("Downloading URL Package (%s)", filepath.Base(url)), resp.Body, hasher); err != nil {
		return "", fmt.Errorf("failed to hash content: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
