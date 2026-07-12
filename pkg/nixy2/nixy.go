package nixy2

import (
	"github.com/nxtcoder17/go.errors"
	"github.com/nxtcoder17/nixy/internal/app"

	"crypto/sha256"
	"fmt"
	yamlAST "github.com/nxtcoder17/nixy/pkg/yaml-ast"
	"gopkg.in/yaml.v3"
	"maps"
	"os"
	"strings"
)

type NixyYAML struct {
	NixPkgs   map[string]string `yaml:"nixpkgs"`
	Packages  []*Package        `yaml:"packages"`
	Libraries []string          `yaml:"libraries,omitempty"`

	Env map[string]string `yaml:"env,omitempty"`

	// OnShellEnter runs at the final step in nixy shell lifecycle
	OnShellEnter string `yaml:"on-shell-enter,omitempty"`

	// OnShellExit is not used as of now, will try to use it in future
	OnShellExit string `yaml:"on-shell-exit,omitempty"`

	Builds map[string]Build `yaml:"builds,omitempty"`

	// Mounts are only for non-local execution modes
	Mounts []NixyMount `yaml:"mounts,omitempty"`

	sha256Sum string `yaml:"-"`

	DockerModeConfig DockerModeConfig `yaml:"docker-mode,omitempty"`
	ColimaModeConfig ColimaModeConfig `yaml:"colima-mode,omitempty"`
}

type ColimaModeConfig struct {
	CPU    int      `yaml:"cpu,omitempty"`
	Memory int      `yaml:"memory,omitempty"`
	Disk   int      `yaml:"disk,omitempty"`
	Ports  []string `yaml:"ports,omitempty"`
}

// NixyMount is for mounting a host file system path to a path in nixy shell
// It is effective only in non-local modes
type NixyMount struct {
	Source      string `yaml:"source"`
	Destination string `yaml:"dest"`

	ReadOnly bool `yaml:"readonly,omitempty"`
}

type Package struct {
	*NixPackage
	*URLPackage
}

type NixPackage struct {
	Name   string
	Commit string
}

type URLPackage struct {
	Name        string                      `yaml:"name"`
	Sources     map[string]PackageURLAndSHA `yaml:"sources"`
	InstallHook string                      `yaml:"installHook,omitempty"`
	BinPaths    []string                    `yaml:"binPaths,omitempty"`
}

type PackageURLAndSHA struct {
	URL    string `yaml:"url"`
	SHA256 string `yaml:"sha256"`
}

type Nixy struct {
	*NixyYAML

	// AUTO FILLED
	sha256Sum string `yaml:"-"`

	fsPaths *FSPaths

	Executor
}

type InNixyShell struct {
	*NixyYAML
	fsPaths *FSPaths
}

func configHash(appCtx *app.Context, content []byte) string {
	h := sha256.New()
	h.Write([]byte(appCtx.AppVersion))
	h.Write([]byte(appCtx.NixyMode))
	h.Write(content)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func fileContentHash(content []byte) string {
	h := sha256.New()
	h.Write(content)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func readNixyFile(file string) ([]byte, *yamlAST.Parser, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, nil, errors.New("failed to read nixy file").Wrap(err).KV("file", file)
	}
	parser, err := yamlAST.NewParser(b)
	if err != nil {
		return nil, nil, err
	}
	return b, parser, nil
}

func decodeNixyYAML(parser *yamlAST.Parser) (*NixyYAML, error) {
	var nw NixyYAML
	if err := parser.Decode(&nw); err != nil {
		return nil, err
	}
	if nw.NixPkgs == nil {
		return nil, errors.New("nixy.yml must have a nixpkgs key")
	}
	if nw.NixPkgs["default"] == "" {
		return nil, errors.New("nixy.yml must have a nixpkgs.default key, containing a nixpkgs hash").
			KV("nixpkgs-map", nw.NixPkgs)
	}

	return &nw, nil
}

func normalizePackages(appCtx *app.Context, nw *NixyYAML, parser *yamlAST.Parser) (bool, error) {
	hasPkgUpdates := false
	osArch := appCtx.OSArch()

	for i, pkg := range nw.Packages {
		if pkg == nil || pkg.URLPackage == nil {
			continue
		}

		v, hasSource := pkg.URLPackage.Sources[osArch]
		if !hasSource || v.URL == "" {
			return false, errors.New("URL package has no source os/arch defined").
				KV("name", pkg.URLPackage.Name, "os/arch", osArch)
		}
		if v.SHA256 != "" {
			continue
		}

		hash, err := fetchURLPackageHash(appCtx, v.URL)
		if err != nil {
			return false, errors.New("failed to fetch SHA256 hash").
				Wrap(err).KV("name", pkg.URLPackage.Name, "url", v.URL)
		}

		hasPkgUpdates = true
		nw.Packages[i].URLPackage.Sources[osArch] = PackageURLAndSHA{URL: v.URL, SHA256: hash}

		if err := parser.SetScalarByPointer(
			fmt.Sprintf("/packages/%d/sources/%s/sha256", i, strings.ReplaceAll(osArch, "/", "~1")),
			hash,
		); err != nil {
			return false, errors.New("failed to apply patches").Wrap(err)
		}
	}

	return hasPkgUpdates, nil
}

func parseAndSyncNixyFile(appCtx *app.Context, file string) (*NixyYAML, error) {
	b, parser, err := readNixyFile(file)
	if err != nil {
		return nil, err
	}

	nw, err := decodeNixyYAML(parser)
	if err != nil {
		return nil, err
	}

	nw.sha256Sum = configHash(appCtx, b)

	hasPkgUpdates, err := normalizePackages(appCtx, nw, parser)
	if err != nil {
		return nil, err
	}

	if hasPkgUpdates {
		if err := nw.syncToDisk(file, parser.Root()); err != nil {
			return nil, err
		}
	}

	return nw, nil
}

// SyncToDisk writes the nixy config to disk.
// When rawNode is set, it preserves the original YAML structure (comments, ordering).
// When rawNode is nil, it encodes the struct with deduplication (for new files).
func (n *NixyYAML) syncToDisk(file string, content any) error {
	if file == "" {
		return fmt.Errorf("required param `file` not provided")
	}

	output, err := os.OpenFile(file,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0o644,
	)
	if err != nil {
		return err
	}
	defer output.Close()

	encoder := yaml.NewEncoder(output)
	encoder.SetIndent(2)
	defer encoder.Close()

	return encoder.Encode(content)
}

func mergeNixyYAMLs(yamls ...*NixyYAML) *NixyYAML {
	n := &NixyYAML{
		NixPkgs: make(map[string]string),
		Env:     make(map[string]string),
		Builds:  make(map[string]Build),
	}

	var shellEnterHooks []string
	var shellExitHooks []string

	for _, y := range yamls {
		if y == nil {
			continue
		}
		maps.Copy(n.NixPkgs, y.NixPkgs)
		n.Packages = append(n.Packages, y.Packages...)
		n.Libraries = append(n.Libraries, y.Libraries...)
		maps.Copy(n.Env, y.Env)
		if y.OnShellEnter != "" {
			shellEnterHooks = append(shellEnterHooks, y.OnShellEnter)
		}
		if y.OnShellExit != "" {
			shellExitHooks = append(shellExitHooks, y.OnShellExit)
		}
		maps.Copy(n.Builds, y.Builds)
		n.Mounts = append(n.Mounts, y.Mounts...)

		n.DockerModeConfig.Ports = append(n.DockerModeConfig.Ports, y.DockerModeConfig.Ports...)
	}

	n.OnShellEnter = strings.Join(shellEnterHooks, "\n")
	n.OnShellExit = strings.Join(shellExitHooks, "\n")

	return n
}

func LoadFromFile(appCtx *app.Context, f string) (*Nixy, error) {
	fsPaths, err := CreateFSPaths(appCtx)
	if err != nil {
		return nil, err
	}

	n, err := func() (*Nixy, error) {
		projectNixy, err := parseAndSyncNixyFile(appCtx, f)
		if err != nil {
			return nil, err
		}

		if _, ok := projectNixy.NixPkgs["default"]; !ok {
			return nil, errors.New("nixy.yml must have a nixpkgs.default key, containing a nixpkgs hash")
		}

		if appCtx.NixyPreload != "" {
			preloadNixy, err := parseAndSyncNixyFile(appCtx, os.ExpandEnv(appCtx.NixyPreload))
			if err != nil {
				return nil, err
			}

			return &Nixy{
				NixyYAML:  mergeNixyYAMLs(preloadNixy, projectNixy),
				fsPaths:   fsPaths,
				sha256Sum: preloadNixy.sha256Sum + ";" + projectNixy.sha256Sum,
				Executor:  nil,
			}, nil
		}

		return &Nixy{
			NixyYAML:  projectNixy,
			fsPaths:   fsPaths,
			sha256Sum: projectNixy.sha256Sum,
			Executor:  nil,
		}, nil
	}()

	if err != nil {
		return nil, err
	}

	if err := n.syncToDisk(fsPaths.GeneratedNixyYAMLPath, n.NixyYAML); err != nil {
		return nil, err
	}

	switch appCtx.NixyMode {
	case app.BubbleWrapMode:
		panic("NOT IMPLEMENTED YET")
	case app.DockerMode:
		n.Executor = n.NewDockerExecutor(appCtx, fsPaths)
	case app.LocalMode:
		n.Executor = n.NewLocalExecutor(appCtx, fsPaths)
	case app.ColimaMode:
		n.Executor = n.NewColimaExecutor(appCtx, fsPaths)
	}

	return n, nil
}

func LoadInNixyShell(appCtx *app.Context) (*InNixyShell, error) {
	fsPaths, err := CreateFSPaths(appCtx)
	if err != nil {
		return nil, err
	}

	b, err := os.ReadFile(fsPaths.GeneratedNixyYAMLPath)
	if err != nil {
		return nil, errors.New("failed to read computed nixy.yml").Wrap(err).KV("path", fsPaths.GeneratedNixyYAMLPath)
	}

	parser, err := yamlAST.NewParser(b)
	if err != nil {
		return nil, err
	}

	var n NixyYAML
	if err := parser.Decode(&n); err != nil {
		return nil, err
	}

	return &InNixyShell{
		NixyYAML: &n,
		fsPaths:  fsPaths,
	}, nil
}
