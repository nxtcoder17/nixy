package templates

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var installPhaseRE = regexp.MustCompile(`(?s)installPhase = ''(.*?)'';`)

func renderInstallPhase(t *testing.T, binaries []string) string {
	t.Helper()

	flake, err := RenderWorkspaceFlake(&WorkspaceFlakeParams{
		NixPkgsCommitsList: []string{"default"},
		NixPkgsCommitsMap:  map[string]string{"default": "abc123"},
		PackagesMap:        map[string][]string{"default": {}},
		LibrariesMap:       map[string][]string{"default": {}},
		URLPackages: []URLPackage{{
			Name:     "tool",
			URL:      "https://example.com/tool.tar.gz",
			Sha256:   "sha256-abc",
			Binaries: binaries,
		}},
		WorkspaceDir:           "/workspace",
		Builds:                 map[string]WorkspaceFlakePackgeBuild{},
		OSArch:                 "linux/arm64",
		EnvVars:                map[string]string{},
		OnShellEnterScriptPath: "/workspace/.nixy/hooks/shell-enter.sh",
	})
	if err != nil {
		t.Fatal(err)
	}

	m := installPhaseRE.FindStringSubmatch(string(flake))
	if len(m) != 2 {
		t.Fatalf("installPhase not found in:\n%s", flake)
	}

	return m[1]
}

func writeFixtureFile(t *testing.T, root, rel string, mode os.FileMode) {
	t.Helper()

	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), mode); err != nil {
		t.Fatal(err)
	}
}

func runInstallPhase(t *testing.T, script, archiveDir, outDir string) (string, error) {
	t.Helper()

	cmd := exec.Command("bash", "-euo", "pipefail", "-c", script)
	cmd.Dir = archiveDir
	cmd.Env = append(os.Environ(), "out="+outDir)

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	return buf.String(), err
}

func assertSymlink(t *testing.T, path, want string) {
	t.Helper()

	got, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("expected symlink at %s: %v", path, err)
	}
	if got != want {
		t.Fatalf("symlink %s -> %s, want %s", path, got, want)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s not to exist (err: %v)", path, err)
	}
}

func autoScanFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFixtureFile(t, root, "root-tool", 0o755)
	writeFixtureFile(t, root, "bin/bin-tool", 0o755)
	writeFixtureFile(t, root, "helpers/helper", 0o755)
	writeFixtureFile(t, root, "README.md", 0o644)
	writeFixtureFile(t, root, "env-vars", 0o644)
	writeFixtureFile(t, root, "deep/a/b/too-deep", 0o755)

	return root
}

func TestInstallPhase_AutoScan_LinksExecutablesUpToDepth3(t *testing.T) {
	fixture := autoScanFixture(t)
	out := t.TempDir()

	log, err := runInstallPhase(t, renderInstallPhase(t, nil), fixture, out)
	if err != nil {
		t.Fatalf("install phase failed: %v\n%s", err, log)
	}

	assertSymlink(t, filepath.Join(out, "bin", "root-tool"), "../package/root-tool")
	assertSymlink(t, filepath.Join(out, "bin", "bin-tool"), "../package/bin/bin-tool")
	assertSymlink(t, filepath.Join(out, "bin", "helper"), "../package/helpers/helper")

	assertMissing(t, filepath.Join(out, "bin", "README.md"))
	assertMissing(t, filepath.Join(out, "bin", "too-deep"))

	if _, err := os.Stat(filepath.Join(out, "package", "README.md")); err != nil {
		t.Fatalf("archive was not preserved under package: %v", err)
	}
	assertMissing(t, filepath.Join(out, "package", "env-vars"))
}

func TestInstallPhase_BinariesDirectory_LinksOnlyExecutableChildren(t *testing.T) {
	fixture := autoScanFixture(t)
	out := t.TempDir()

	log, err := runInstallPhase(t, renderInstallPhase(t, []string{"./bin"}), fixture, out)
	if err != nil {
		t.Fatalf("install phase failed: %v\n%s", err, log)
	}

	assertSymlink(t, filepath.Join(out, "bin", "bin-tool"), "../package/bin/bin-tool")
	assertMissing(t, filepath.Join(out, "bin", "root-tool"))
	assertMissing(t, filepath.Join(out, "bin", "helper"))
}

func TestInstallPhase_BinariesDot_LinksRootExecutables(t *testing.T) {
	fixture := autoScanFixture(t)
	out := t.TempDir()

	log, err := runInstallPhase(t, renderInstallPhase(t, []string{"."}), fixture, out)
	if err != nil {
		t.Fatalf("install phase failed: %v\n%s", err, log)
	}

	assertSymlink(t, filepath.Join(out, "bin", "root-tool"), "../package/root-tool")
	assertMissing(t, filepath.Join(out, "bin", "bin-tool"))
	assertMissing(t, filepath.Join(out, "bin", "helper"))
}

func TestInstallPhase_BinariesFile_MarksItExecutable(t *testing.T) {
	fixture := autoScanFixture(t)
	writeFixtureFile(t, fixture, "scripts/run", 0o644)
	out := t.TempDir()

	log, err := runInstallPhase(t, renderInstallPhase(t, []string{"./scripts/run"}), fixture, out)
	if err != nil {
		t.Fatalf("install phase failed: %v\n%s", err, log)
	}

	linked := filepath.Join(out, "bin", "run")
	assertSymlink(t, linked, "../package/scripts/run")

	info, err := os.Stat(linked)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("linked binary is not executable: %v", info.Mode())
	}
}

func TestInstallPhase_BinariesMissing_ReportsNoMatch(t *testing.T) {
	fixture := autoScanFixture(t)
	out := t.TempDir()

	log, err := runInstallPhase(t, renderInstallPhase(t, []string{"./missing"}), fixture, out)
	if err == nil {
		t.Fatalf("expected a missing binaries entry to fail:\n%s", log)
	}
	if !strings.Contains(log, "matched nothing") {
		t.Fatalf("log does not explain the missing entry:\n%s", log)
	}
}

func TestInstallPhase_BinariesDirWithoutExecutables_ReportsNoExecutables(t *testing.T) {
	fixture := autoScanFixture(t)
	writeFixtureFile(t, fixture, "docs/README.md", 0o644)
	out := t.TempDir()

	log, err := runInstallPhase(t, renderInstallPhase(t, []string{"./docs"}), fixture, out)
	if err == nil {
		t.Fatalf("expected a directory with no executables to fail:\n%s", log)
	}
	if !strings.Contains(log, "matched no executables") {
		t.Fatalf("log does not explain the empty directory:\n%s", log)
	}
}

func TestInstallPhase_DuplicateBasename_ReportsCollision(t *testing.T) {
	fixture := t.TempDir()
	writeFixtureFile(t, fixture, "a/tool", 0o755)
	writeFixtureFile(t, fixture, "b/tool", 0o755)
	out := t.TempDir()

	log, err := runInstallPhase(t, renderInstallPhase(t, nil), fixture, out)
	if err == nil {
		t.Fatalf("expected duplicate basenames to fail:\n%s", log)
	}
	if !strings.Contains(log, "two executables are named 'tool'") {
		t.Fatalf("log does not explain the collision:\n%s", log)
	}
}
