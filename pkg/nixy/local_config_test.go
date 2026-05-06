package nixy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFromFileLoadsLocalConfigWithoutNixpkgsDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "xdg-data"))
	t.Setenv("NIXY_EXECUTOR", "test")

	projectConfig := []byte(`nixpkgs:
  default: test-default
  shared: project-shared
packages:
  - go
env:
  PROJECT_ENV: project
`)
	if err := os.WriteFile(filepath.Join(tmp, "nixy.yml"), projectConfig, 0o644); err != nil {
		t.Fatal(err)
	}

	localConfig := []byte(`packages:
  - ripgrep
nixpkgs:
  default: local-default
  shared: local-shared
  local: local-only
env:
  LOCAL_ENV: local
onShellEnter: |
  export LOCAL_SHELL=1
`)
	if err := os.WriteFile(filepath.Join(tmp, "nixy.local.yml"), localConfig, 0o644); err != nil {
		t.Fatal(err)
	}

	nixy, err := LoadFromFile(context.Background(), filepath.Join(tmp, "nixy.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if nixy.localNixy == nil {
		t.Fatal("expected nixy.local.yml to be loaded")
	}

	nixy.useLocalNixy = true
	packages := nixy.shellPackages(nil)
	if len(packages) != 2 {
		t.Fatalf("expected local and project packages, got %d", len(packages))
	}
	if packages[0].NixPackage.Name != "ripgrep" || packages[1].NixPackage.Name != "go" {
		t.Fatalf("expected local package before project package, got %#v", packages)
	}

	env := nixy.shellEnvVars(nixy.Context)
	if env["LOCAL_ENV"] != "local" || env["PROJECT_ENV"] != "project" {
		t.Fatalf("expected local and project env vars, got %#v", env)
	}

	nixpkgs := nixy.shellNixPkgs()
	if nixpkgs["default"] != "test-default" {
		t.Fatalf("expected project default nixpkgs to win, got %q", nixpkgs["default"])
	}
	if nixpkgs["shared"] != "project-shared" {
		t.Fatalf("expected project named nixpkgs to win, got %q", nixpkgs["shared"])
	}
	if nixpkgs["local"] != "local-only" {
		t.Fatalf("expected local-only nixpkgs to be preserved, got %q", nixpkgs["local"])
	}
}

func TestWriteMergedNixyYAMLIncludesEffectiveLocalConfig(t *testing.T) {
	nixy := &Nixy{
		NixPkgs: NixPkgsMap{"default": "project"},
		Packages: []*NormalizedPackage{
			{NixPackage: &NixPackage{Name: "ripgrep"}},
			{NixPackage: &NixPackage{Name: "go"}},
		},
		Env: map[string]string{"LOCAL_ENV": "local"},
	}

	path := filepath.Join(t.TempDir(), "nixy-merged.yml")
	if err := writeMergedNixyYAML(path, nixy); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(b)
	for _, expected := range []string{"nixpkgs:", "ripgrep", "go", "LOCAL_ENV: local"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("expected %q in nixy-merged.yml, got:\n%s", expected, content)
		}
	}
}
