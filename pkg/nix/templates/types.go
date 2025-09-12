package templates

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed profile-nixy.yml.tpl
var profileNixyYamlContent string

//go:embed workspace-flake.nix.tpl
var wsFlakeContent string

//go:embed shell-hook.sh.tpl
var shellHookScript string

//go:embed build-hook.sh.tpl
var buildHookScript string

//go:embed nix.conf.tpl
var nixConf string

var t *template.Template

func init() {
	t = template.New("templates")

	t.Funcs(template.FuncMap{
		"hasKey": func(item map[string]any, key string) bool {
			if _, ok := item[key]; ok {
				return true
			}
			return false
		},
		"hasPrefix": strings.HasPrefix,
	})

	if _, err := t.Parse(profileNixyYamlContent); err != nil {
		panic(fmt.Errorf("failed to parse profile nixy.yml: %w", err))
	}
	if _, err := t.Parse(wsFlakeContent); err != nil {
		panic(fmt.Errorf("failed to parse workspace flake.nix: %w", err))
	}

	if _, err := t.Parse(shellHookScript); err != nil {
		panic(fmt.Errorf("failed to parse shell hook script: %w", err))
	}

	if _, err := t.Parse(buildHookScript); err != nil {
		panic(fmt.Errorf("failed to parse build hook script: %w", err))
	}

	if _, err := t.Parse(nixConf); err != nil {
		panic(fmt.Errorf("failed to parse nix conf: %w", err))
	}
}

// copy of pkg/nix.URLPackage
type URLPackage struct {
	Name   string `yaml:"name"`
	URL    string `yaml:"url"`
	Sha256 string `yaml:"sha256,omitempty"`
}

type WorkspaceFlakeParams struct {
	NixPkgsDefaultCommit string
	NixPkgsCommits       []string

	PackagesMap  map[string][]string
	LibrariesMap map[string][]string
	URLPackages  []URLPackage

	WorkspaceDir string

	Builds map[string]WorkspaceFlakePackgeBuild

	UseProfile  bool
	ProfilePath string
}

type WorkspaceFlakePackgeBuild struct {
	PackagesMap map[string][]string
	Paths       []string
}

func RenderWorkspaceFlake(values *WorkspaceFlakeParams) ([]byte, error) {
	b := new(bytes.Buffer)
	if err := t.ExecuteTemplate(b, "workspace-flake", values); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

type ProfileNixyYAMLParams struct {
	NixPkgsCommit string
}

func RenderProfileNixyYAML(values ProfileNixyYAMLParams) ([]byte, error) {
	b := new(bytes.Buffer)
	if err := t.ExecuteTemplate(b, "profile-nixy.yml", values); err != nil {
		return nil, fmt.Errorf("failed to render profile's nixy.yml: %w", err)
	}

	return b.Bytes(), nil
}

type ShellHookParams struct {
	EnvVars   map[string]string
	ShellHook string
}

func RenderShellHook(params ShellHookParams) ([]byte, error) {
	b := new(bytes.Buffer)
	if err := t.ExecuteTemplate(b, "shell-hook", params); err != nil {
		return nil, fmt.Errorf("failed to render shell-hook.sh: %w", err)
	}

	return b.Bytes(), nil
}

type BuildHookParams struct {
	ProjectDir  string
	BuildTarget string
	CopyPaths   []string
}

func RenderBuildHook(params BuildHookParams) ([]byte, error) {
	b := new(bytes.Buffer)
	if err := t.ExecuteTemplate(b, "build-hook", params); err != nil {
		return nil, fmt.Errorf("failed to render build-hook.sh: %w", err)
	}

	return b.Bytes(), nil
}

func RenderNixConf() ([]byte, error) {
	b := new(bytes.Buffer)
	if err := t.ExecuteTemplate(b, "nix.conf", nil); err != nil {
		return nil, fmt.Errorf("failed to render nix.conf: %w", err)
	}

	return b.Bytes(), nil
}
