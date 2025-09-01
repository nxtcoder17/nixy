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

var wsFlake *template.Template

func init() {
	wsFlake = template.New("workspace-flake")
	wsFlake.Funcs(template.FuncMap{
		"hasKey": func(item map[string]any, key string) bool {
			if _, ok := item[key]; ok {
				return true
			}
			return false
		},
		"hasPrefix": strings.HasPrefix,
	})
	if _, err := wsFlake.Parse(wsFlakeContent); err != nil {
		panic(err)
	}
}

func RenderWorkspaceFlake(values any) ([]byte, error) {
	b := new(bytes.Buffer)
	if err := wsFlake.ExecuteTemplate(b, "workspace-flake", values); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

func RenderProfileFlake(values any) ([]byte, error) {
	t := template.New("profile-flake")
	if _, err := t.Parse(profileFlakeContent); err != nil {
		return nil, fmt.Errorf("failed to parse profile's flake.nix: %w", err)
	}

	b := new(bytes.Buffer)
	if err := t.ExecuteTemplate(b, "profile-flake", values); err != nil {
		return nil, fmt.Errorf("failed to render profile's flake.nix: %w", err)
	}

	return b.Bytes(), nil
}
