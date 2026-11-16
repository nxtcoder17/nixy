package app

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestEnvVars_EveryVarHasUsage(t *testing.T) {
	for _, v := range EnvVars() {
		if strings.TrimSpace(v.Usage) == "" {
			t.Errorf("%s has no usage tag, so `nixy env` cannot describe it", v.Name)
		}
	}
}

func TestEnvVars_NonConfigurableFields_AreExcluded(t *testing.T) {
	names := make([]string, 0, len(EnvVars()))
	for _, v := range EnvVars() {
		names = append(names, v.Name)
	}

	if len(names) == 0 {
		t.Fatal("EnvVars() returned nothing")
	}
	for _, name := range names {
		if name == "-" {
			t.Fatalf("listed a non-configurable field: %v", names)
		}
	}
	for _, want := range []string{"NIXY_MODE", "NIXY_SHELL", "NIXY_DISABLE_WORKTREE_MOUNTS"} {
		if !slices.Contains(names, want) {
			t.Errorf("EnvVars() missing %s, got %v", want, names)
		}
	}
}

func TestEnvVars_SetVar_ReportsValueAndDefault(t *testing.T) {
	t.Setenv("NIXY_MODE", "docker")

	for _, v := range EnvVars() {
		if v.Name != "NIXY_MODE" {
			continue
		}
		if v.Value != "docker" {
			t.Fatalf("NIXY_MODE value = %q, want %q", v.Value, "docker")
		}
		if v.Default != "local" {
			t.Fatalf("NIXY_MODE default = %q, want %q", v.Default, "local")
		}
		return
	}

	t.Fatal("EnvVars() missing NIXY_MODE")
}

func TestLoadEnv_DisableWorktreeMounts(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "true disables worktree mounts", value: "true", want: true},
		{name: "false keeps worktree mounts", value: "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NIXY_DISABLE_WORKTREE_MOUNTS", tt.value)

			e, err := LoadEnv()
			if err != nil {
				t.Fatal(err)
			}
			if e.NixyDisableWorktreeMounts != tt.want {
				t.Fatalf("NixyDisableWorktreeMounts = %v, want %v", e.NixyDisableWorktreeMounts, tt.want)
			}
		})
	}
}

func TestLoadEnv_GlobalDir_DefaultsToXDG(t *testing.T) {
	t.Setenv("NIXY_GLOBAL_DIR", "")
	t.Setenv("XDG_DATA_HOME", filepath.Join("/xdg", "data"))

	e, err := LoadEnv()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/xdg", "data", "nixy"); e.NixyGlobalDir != want {
		t.Fatalf("NixyGlobalDir = %q, want %q", e.NixyGlobalDir, want)
	}
}

func TestLoadEnv_GlobalDir_SetIsHonoured(t *testing.T) {
	t.Setenv("NIXY_GLOBAL_DIR", filepath.Join("/custom", "nixy"))
	t.Setenv("XDG_DATA_HOME", "/xdg")

	e, err := LoadEnv()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/custom", "nixy"); e.NixyGlobalDir != want {
		t.Fatalf("NixyGlobalDir = %q, want %q", e.NixyGlobalDir, want)
	}
}
