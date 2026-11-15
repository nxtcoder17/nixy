package nixy2

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nxtcoder17/nixy/internal/app"
)

func TestParseAndSyncNixyFile_ColimaArch_SelectsVMPlatformSource(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "nixy.yml")
	content := `nixpkgs:
  default: abc123
colima-mode:
  arch: x86_64
packages:
  - name: tool
    sources:
      linux/amd64:
        url: https://example.com/tool-amd64
        sha256: amd64hash
      linux/arm64:
        url: https://example.com/tool-arm64
        sha256: arm64hash
`
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	appCtx := &app.Context{
		Context:    context.Background(),
		Env:        &app.Env{NixyMode: app.ColimaMode},
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
	if got.URLPackages[0].URL != "https://example.com/tool-amd64" {
		t.Errorf("selected URL = %q, want the linux/amd64 source", got.URLPackages[0].URL)
	}
}
