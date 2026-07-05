package nixy2

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nxtcoder17/nixy/internal/app"
)

type mockExecutor struct {
	executedCmd  string
	executedArgs []string
}

func (m *mockExecutor) Exec(appCtx *app.Context, cmd string, args ...string) (*exec.Cmd, error) {
	m.executedCmd = cmd
	m.executedArgs = args
	return exec.CommandContext(appCtx.Context, "echo", "mocked"), nil
}

func TestBuild_Paths(t *testing.T) {
	b := Build{
		Dir: "src/subdir",
	}

	if b.ResolvedDir() != "src/subdir" {
		t.Errorf("expected ResolvedDir to be 'src/subdir', got '%s'", b.ResolvedDir())
	}

	bEmpty := Build{}
	if bEmpty.ResolvedDir() != "." {
		t.Errorf("expected empty resolved dir to be '.', got '%s'", bEmpty.ResolvedDir())
	}

	pwd := "/workspace"
	if b.BuildDir(pwd) != "/workspace/src/subdir" {
		t.Errorf("unexpected BuildDir: %s", b.BuildDir(pwd))
	}
	if b.OutputDir(pwd) != "/workspace/src/subdir/.nixy/dist" {
		t.Errorf("unexpected OutputDir: %s", b.OutputDir(pwd))
	}
}

func TestInNixyShell_Build(t *testing.T) {
	tmpDir := t.TempDir()

	// Write mock nix binary
	binDir := filepath.Join(tmpDir, "bin")
	err := os.MkdirAll(binDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	mockNixContent := `#!/bin/sh
if [ "$1" = "build" ]; then
  shift
  shift
  while [ "$#" -gt 0 ]; do
    if [ "$1" = "-o" ]; then
      out_dir="$2"
      mkdir -p "$out_dir"
      echo "dummy" > "$out_dir/app"
      break
    fi
    shift
  done
elif [ "$1" = "path-info" ]; then
  echo "/tmp"
fi
exit 0
`
	mockNixPath := filepath.Join(binDir, "nix")
	err = os.WriteFile(mockNixPath, []byte(mockNixContent), 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Update PATH to include mock nix
	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+originalPath)

	appCtx := &app.Context{
		Context: context.Background(),
		PWD:     tmpDir,
		Env: &app.Env{
			NixyProjectDir: ".nixy",
			NixyGlobalDir:  filepath.Join(tmpDir, "global"),
		},
	}

	fsPaths, err := CreateFSPaths(appCtx)
	if err != nil {
		t.Fatal(err)
	}

	inShell := &InNixyShell{
		NixyYAML: &NixyYAML{
			Builds: map[string]Build{
				"test-target": {
					Dir:     "subdir",
					Command: "echo 'building'",
					Paths:   []string{"dist/app"},
				},
			},
		},
		fsPaths: fsPaths,
	}

	// Create the build subdir and the paths to avoid shell errors
	err = os.MkdirAll(filepath.Join(tmpDir, "subdir", "dist"), 0755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(tmpDir, "subdir", "dist", "app"), []byte("app binary"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create scripts dir since it writes buildScript there
	err = os.MkdirAll(filepath.Join(fsPaths.GeneratedArtifactsDir, "scripts"), 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Run Build
	err = inShell.Build(appCtx, "test-target")
	if err != nil {
		t.Fatalf("unexpected error during build: %v", err)
	}

	// Verify the script was written
	scriptPath := filepath.Join(fsPaths.GeneratedArtifactsDir, "scripts", "build-test-target")
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		t.Errorf("expected build script to exist at %s", scriptPath)
	}

	b, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	scriptContent := string(b)
	if !strings.Contains(scriptContent, "echo 'building'") {
		t.Errorf("expected command 'echo building' in script, got:\n%s", scriptContent)
	}

	// Test non-existent target
	err = inShell.Build(appCtx, "invalid-target")
	if err == nil {
		t.Error("expected error for non-existent target, got nil")
	}
}

func TestNixy_Build(t *testing.T) {
	tmpDir := t.TempDir()

	// Set Env variables for NixyProjectDir
	appCtx := &app.Context{
		Context: context.Background(),
		PWD:     tmpDir,
		Env: &app.Env{
			NixyProjectDir: ".nixy",
			NixyGlobalDir:  filepath.Join(tmpDir, "global"),
		},
	}

	fsPaths, err := CreateFSPaths(appCtx)
	if err != nil {
		t.Fatal(err)
	}

	mockExec := &mockExecutor{}

	nw := &Nixy{
		NixyYAML: &NixyYAML{
			NixPkgs: map[string]string{"default": "test-commit"},
			Builds: map[string]Build{
				"test-target": {
					Dir:     "subdir",
					Command: "echo 'building'",
					Paths:   []string{"dist/app"},
				},
			},
		},
		fsPaths:  fsPaths,
		Executor: mockExec,
	}

	err = nw.Build(appCtx, "test-target")
	if err != nil {
		t.Fatalf("unexpected error during build: %v", err)
	}

	// Check that the script file was written to the flake path (.nixy)
	scriptPath := filepath.Join(tmpDir, ".nixy", "build.test-target.sh")
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		t.Errorf("expected build script to exist at %s", scriptPath)
	}

	// Check if mock executor was called with the bash execution
	if mockExec.executedCmd != "nix" {
		t.Errorf("expected exec command to be 'nix', got '%s'", mockExec.executedCmd)
	}

	// The args passed to Exec should contain "bash .nixy/build.test-target.sh"
	found := false
	for _, arg := range mockExec.executedArgs {
		if strings.Contains(arg, "bash .nixy/build.test-target.sh") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected args to contain 'bash .nixy/build.test-target.sh', got: %v", mockExec.executedArgs)
	}
}
