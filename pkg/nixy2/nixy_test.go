package nixy2

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/nxtcoder17/nixy/internal/app"
)

func TestParseAndSyncNixyFile_PlatformConfig_SelectsMatchingSource(t *testing.T) {
	tests := []struct {
		name       string
		mode       app.NixyMode
		modeConfig string
		wantURL    string
	}{
		{
			name:       "colima VM arch",
			mode:       app.ColimaMode,
			modeConfig: "colima-mode:\n  arch: x86_64",
			wantURL:    "https://example.com/tool-amd64",
		},
		{
			name:       "docker platform",
			mode:       app.DockerMode,
			modeConfig: "docker-mode:\n  platform: linux/amd64",
			wantURL:    "https://example.com/tool-amd64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "nixy.yml")
			content := fmt.Sprintf(`nixpkgs:
  default: abc123
%s
packages:
  - name: tool
    sources:
      linux/amd64:
        url: https://example.com/tool-amd64
        sha256: amd64hash
      linux/arm64:
        url: https://example.com/tool-arm64
        sha256: arm64hash
`, tt.modeConfig)
			if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}

			appCtx := &app.Context{
				Context:    context.Background(),
				Env:        &app.Env{NixyMode: tt.mode},
				SystemOS:   "darwin",
				SystemARCH: "arm64",
			}

			nw, err := parseAndSyncNixyFile(appCtx, file)
			if err != nil {
				t.Fatal(err)
			}

			got, err := genWorkspaceFlakeParams(appCtx, WorkspaceFlakeGenParams{
				NixPkgs:          nw.NixPkgs,
				WorkspaceDirPath: dir,
				Packages:         nw.Packages,
				FSPaths:          &FSPaths{GeneratedHookOnShellEnterPath: filepath.Join(dir, ".nixy", "hooks", "shell-enter.sh")},
			})
			if err != nil {
				t.Fatal(err)
			}

			if len(got.URLPackages) != 1 {
				t.Fatalf("expected 1 URL package, got %d", len(got.URLPackages))
			}
			if got.URLPackages[0].URL != tt.wantURL {
				t.Errorf("selected URL = %q, want %q", got.URLPackages[0].URL, tt.wantURL)
			}
		})
	}
}
