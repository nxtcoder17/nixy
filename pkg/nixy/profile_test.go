package nixy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewProfileCreatesProfileYAMLOnly(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "xdg-data"))

	runtimePaths, err := NewRuntimePaths("test")
	if err != nil {
		t.Fatal(err)
	}

	profile, err := NewProfile(&Context{Context: context.Background()}, "test", runtimePaths)
	if err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(profile.ProfilePath, "profile.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "nixpkgs:") || strings.Contains(string(b), "default:") {
		t.Fatalf("expected scalar nixpkgs in profile.yaml, got:\n%s", b)
	}
	if _, err := os.Stat(filepath.Join(profile.ProfilePath, "nixy.yml")); !os.IsNotExist(err) {
		t.Fatalf("expected profile nixy.yml to not exist, got err=%v", err)
	}
}
