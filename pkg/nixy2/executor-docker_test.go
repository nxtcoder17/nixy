package nixy2

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/nxtcoder17/nixy/internal/app"
)

func TestDockerExecutor_PlatformFlag_AddsFlagOnlyWhenSet(t *testing.T) {
	platform := "linux/amd64"
	tests := []struct {
		name     string
		platform *string
		want     string
	}{
		{name: "unset"},
		{name: "configured", platform: &platform, want: platform},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(binDir, "docker"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

			appCtx := &app.Context{
				Context: context.Background(),
				Env: &app.Env{
					NixyMode:              app.DockerMode,
					NixyDockerModeImage:   "nixy-test",
					NixyExecutableBinPath: "/bin/nixy",
				},
				PWD: t.TempDir(),
			}
			n := &Nixy{
				NixyYAML: &NixyYAML{
					DockerModeConfig: DockerModeConfig{Platform: tt.platform},
				},
			}

			cmd, err := n.NewDockerExecutor(appCtx, &FSPaths{}).Exec(appCtx, "nix", "--version")
			if err != nil {
				t.Fatal(err)
			}

			idx := slices.Index(cmd.Args, "--platform")
			if tt.want == "" {
				if idx >= 0 {
					t.Fatalf("unexpected --platform in args: %v", cmd.Args)
				}
				return
			}
			if idx < 0 || idx+1 >= len(cmd.Args) || cmd.Args[idx+1] != tt.want {
				t.Fatalf("--platform %q missing from args: %v", tt.want, cmd.Args)
			}
		})
	}
}
