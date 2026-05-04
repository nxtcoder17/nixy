package templates

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	errors "github.com/nxtcoder17/go.errors"
	"strings"
	"text/template"
)

//go:embed profile-nixy.yml.tpl
var profileNixyYamlContent string

//go:embed workspace-flake.nix.tpl
var wsFlakeContent string

//go:embed shell-enter.sh.tpl
var shellEnterScript string

//go:embed build.sh.tpl
var buildScript string

//go:embed nix.conf.tpl
var nixConf string

//go:embed build-dockerfile.tpl
var dockerfileTemplate string

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
		"toJson": func(v any) (string, error) {
			b, err := json.Marshal(v)
			return string(b), err
		},
		"squote": func(str ...interface{}) string {
			out := make([]string, 0, len(str))
			for _, s := range str {
				if s != nil {
					out = append(out, fmt.Sprintf("'%v'", s))
				}
			}
			return strings.Join(out, " ")
		},
	})

	if _, err := t.Parse(profileNixyYamlContent); err != nil {
		panic(fmt.Errorf("failed to parse profile nixy.yml: %w", err))
	}
	if _, err := t.Parse(wsFlakeContent); err != nil {
		panic(fmt.Errorf("failed to parse workspace flake.nix: %w", err))
	}

	if _, err := t.Parse(shellEnterScript); err != nil {
		panic(fmt.Errorf("failed to parse shell enter script: %w", err))
	}

	if _, err := t.Parse(buildScript); err != nil {
		panic(fmt.Errorf("failed to parse build script: %w", err))
	}

	if _, err := t.Parse(nixConf); err != nil {
		panic(fmt.Errorf("failed to parse nix conf: %w", err))
	}

	if _, err := t.Parse(dockerfileTemplate); err != nil {
		panic(fmt.Errorf("failed to parse nixy's build dockerfile template: %w", err))
	}
}

// copy of pkg/nix.URLPackage
type URLPackage struct {
	Name        string `yaml:"name"`
	URL         string `yaml:"url"`
	Sha256      string `yaml:"sha256"`
	InstallHook string
	BinPaths    []string `yaml:"-"`
}

type WorkspaceFlakeParams struct {
	NixPkgsCommitsList []string
	NixPkgsCommitsMap  map[string]string

	PackagesMap  map[string][]string
	LibrariesMap map[string][]string
	URLPackages  []URLPackage

	WorkspaceDir string

	Builds map[string]WorkspaceFlakePackgeBuild

	OSArch string

	EnvVars map[string]string
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
	OnShellEnter string
}

func RenderShellEnter(params ShellHookParams) ([]byte, error) {
	b := new(bytes.Buffer)
	if err := t.ExecuteTemplate(b, "shell-enter", params); err != nil {
		return nil, fmt.Errorf("failed to render shell-enter.sh: %w", err)
	}

	return b.Bytes(), nil
}

type BuildHookParams struct {
	WorkDir     string
	BuildTarget string
	OutputDir   string
	CopyPaths   []string
	Command     string
}

func RenderBuildScript(params BuildHookParams) ([]byte, error) {
	b := new(bytes.Buffer)
	if err := t.ExecuteTemplate(b, "build", params); err != nil {
		return nil, fmt.Errorf("failed to render build script: %w", err)
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

func RenderDockerfile() ([]byte, error) {
	b := new(bytes.Buffer)

	if err := t.ExecuteTemplate(b, "build-dockerfile", nil); err != nil {
		return nil, errors.New("failed to render build dockerfile").Wrap(err)
	}

	return b.Bytes(), nil
}
