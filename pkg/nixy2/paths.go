package nixy2

import (
	"bytes"
	"context"
	"github.com/nxtcoder17/fastlog"
	"github.com/nxtcoder17/go.errors"
	"github.com/nxtcoder17/nixy/internal/app"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type FSPaths struct {
	NixStoreDir string
	UserHomeDir string

	GeneratedArtifactsDir       string
	GeneratedNixConfigFilePath  string
	GeneratedConfigHashFilePath string
	GeneratedNixyYAMLPath       string
	GeneratedFlakeNixPath       string
	GeneratedHooksDir           string

	GeneratedHookOnShellEnterPath string
}

func GetGitRootForWorkspace(ctx context.Context, dir string) (string, bool) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	result, err := cmd.CombinedOutput()
	if err != nil {
		fastlog.Debug("[CHECK/git-root] show-toplevel (FAILED)", "cmd", cmd.String(), "stderr", string(result), "err", err)
		return "", false
	}

	gitRoot := string(bytes.TrimSpace(result))

	fastlog.Debug("[CHECK/git-root]", "git-root", gitRoot)
	if strings.HasSuffix(gitRoot, "/.git") {
		return gitRoot[:len(gitRoot)-len("/.git")], true
	}

	return gitRoot, true
}

func CreateFSPaths(appCtx *app.Context) (*FSPaths, error) {
	// /nix/var/nix is path for a directory to be used for nix store

	fsPaths := &FSPaths{
		NixStoreDir:           filepath.Join(appCtx.NixyGlobalDir, "nix"),
		UserHomeDir:           filepath.Join(appCtx.NixyGlobalDir, "home"),
		GeneratedArtifactsDir: filepath.Join(appCtx.PWD, appCtx.NixyProjectDir),
	}

	fsPaths.GeneratedNixConfigFilePath = filepath.Join(fsPaths.GeneratedArtifactsDir, "nix.conf")
	fsPaths.GeneratedNixyYAMLPath = filepath.Join(fsPaths.GeneratedArtifactsDir, "nixy.yml")
	fsPaths.GeneratedFlakeNixPath = filepath.Join(fsPaths.GeneratedArtifactsDir, "flake.nix")
	fsPaths.GeneratedHooksDir = filepath.Join(fsPaths.GeneratedArtifactsDir, "hooks")

	fsPaths.GeneratedHookOnShellEnterPath = filepath.Join(fsPaths.GeneratedHooksDir, "shell-enter.sh")
	fsPaths.GeneratedConfigHashFilePath = filepath.Join(fsPaths.GeneratedArtifactsDir, "nixy.sha256")

	if err := createDirs(
		fsPaths.NixStoreDir,
		fsPaths.UserHomeDir,
		fsPaths.GeneratedArtifactsDir,
		fsPaths.GeneratedHooksDir,
	); err != nil {
		return nil, err
	}

	return fsPaths, nil
}

func createDirs(dirs ...string) error {
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return nil
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	if err == nil {
		return true
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	return false
}

func genCleanPathName(name string) string {
	result := make([]byte, len(name))

	for i, c := range []byte(name) {
		if ('a' <= c && c <= 'z') ||
			('A' <= c && c <= 'Z') ||
			('0' <= c && c <= '9') {
			result[i] = c
		} else {
			result[i] = '-'
		}
	}

	return string(result)
}
