package templates

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed profile-flake.nix.tpl
var profileFlakeContent string

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

	if _, err := t.Parse(profileFlakeContent); err != nil {
		panic(fmt.Errorf("failed to parse profile flake.nix: %w", err))
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

func RenderWorkspaceFlake(values any) ([]byte, error) {
	b := new(bytes.Buffer)
	if err := t.ExecuteTemplate(b, "workspace-flake", values); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

func RenderProfileFlake(values any) ([]byte, error) {
	b := new(bytes.Buffer)
	if err := t.ExecuteTemplate(b, "profile-flake", values); err != nil {
		return nil, fmt.Errorf("failed to render profile's flake.nix: %w", err)
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
