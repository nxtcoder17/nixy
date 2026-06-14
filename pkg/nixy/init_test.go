package nixy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitWorkspaceCreatesLocalConfigAndGitignoreEntries(t *testing.T) {
	tmp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	})

	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "xdg-data"))

	if err := os.WriteFile("nixy.yml", []byte("nixpkgs:\n  default: test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".gitignore", []byte("bin/\n.nixy/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldConfirm := confirmInitWorkspace
	confirmInitWorkspace = func(string) bool { return true }
	t.Cleanup(func() { confirmInitWorkspace = oldConfirm })

	if err := InitWorkspace(context.Background()); err != nil {
		t.Fatal(err)
	}

	localConfig, err := os.ReadFile("nixy.local.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(localConfig), "packages: []") || !strings.Contains(string(localConfig), "nixpkgs: {}") {
		t.Fatalf("expected nixy.local.yml template, got:\n%s", localConfig)
	}

	gitignore, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(gitignore), ".nixy/"); got != 1 {
		t.Fatalf("expected .nixy/ once, got %d in:\n%s", got, gitignore)
	}
	if !strings.Contains(string(gitignore), "nixy.local.yml\n") {
		t.Fatalf("expected nixy.local.yml in .gitignore, got:\n%s", gitignore)
	}

	if _, err := os.Stat(filepath.Join(tmp, "nixy.yml")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tmp, ".nixy")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "xdg-data", "nixy", "profiles", "default")); err != nil {
		t.Fatal(err)
	}
}

func TestInitWorkspaceSkipsChangesWhenUserDeclines(t *testing.T) {
	tmp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	})

	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "xdg-data"))

	oldConfirm := confirmInitWorkspace
	confirmInitWorkspace = func(string) bool { return false }
	t.Cleanup(func() { confirmInitWorkspace = oldConfirm })

	if err := InitWorkspace(context.Background()); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"nixy.yml", "nixy.local.yml", ".gitignore", ".nixy", "xdg-data"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s to not exist after declined init", path)
		}
	}
}
