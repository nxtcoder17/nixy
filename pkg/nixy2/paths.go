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

	GeneratedArtifactsDir         string
	GeneratedNixConfigFilePath    string
	GeneratedConfigHashFilePath   string
	GeneratedNixyYAMLPath         string
	GeneratedFlakeNixPath         string
	GeneratedHooksDir             string
	GeneratedColimaConfigFilePath string

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

// listGitWorktrees returns the absolute path of every worktree of the repo
// containing dir, main worktree included.
func listGitWorktrees(ctx context.Context, dir string) []string {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "worktree", "list", "--porcelain")
	result, err := cmd.CombinedOutput()
	if err != nil {
		fastlog.Debug("[CHECK/git-worktrees] worktree list (FAILED)", "cmd", cmd.String(), "stderr", string(result), "err", err)
		return nil
	}

	return parseGitWorktreeList(result)
}

func parseGitWorktreeList(out []byte) []string {
	var dirs []string
	for _, line := range strings.Split(string(out), "\n") {
		path, ok := strings.CutPrefix(line, "worktree ")
		if !ok {
			continue
		}
		if path = strings.TrimSpace(path); path != "" {
			dirs = append(dirs, path)
		}
	}

	return dirs
}

// commonDirPrefix returns the deepest directory containing every dir.
func commonDirPrefix(dirs []string) string {
	if len(dirs) == 0 {
		return ""
	}

	prefix := filepath.Clean(dirs[0])
	for _, dir := range dirs[1:] {
		dir = filepath.Clean(dir)
		for !isSameOrUnder(dir, prefix) {
			parent := filepath.Dir(prefix)
			if parent == prefix {
				return prefix
			}
			prefix = parent
		}
	}

	return prefix
}

func isSameOrUnder(path, dir string) bool {
	return path == dir || strings.HasPrefix(path, dir+string(os.PathSeparator))
}

// isSafeMountRoot rejects roots that would drag unrelated work into the
// executor, e.g. / or the whole home directory.
func isSafeMountRoot(root string) bool {
	if root == "" || root == string(os.PathSeparator) {
		return false
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return true
	}

	home = filepath.Clean(home)
	return root != home && !isSameOrUnder(home, root)
}

// GetGitWorktreesParentDir returns the directory shared by every worktree of
// the workspace. Mounting it must keep the original path: a worktree's .git
// file points at the shared git dir by absolute path.
//
// Worktrees that share no safe dir with the repo, e.g. a bare repo whose
// worktrees live outside it, are unsupported and report false.
func GetGitWorktreesParentDir(ctx context.Context, dir string) (string, bool) {
	dirs := listGitWorktrees(ctx, dir)
	if len(dirs) < 2 {
		return "", false
	}

	parent := commonDirPrefix(dirs)
	if !isSafeMountRoot(parent) {
		fastlog.Warn("[CHECK/git-worktrees] worktrees mount parent is unsafe, skipping", "parent", parent, "worktrees", dirs)
		return "", false
	}

	fastlog.Debug("[CHECK/git-worktrees]", "parent", parent, "worktrees", dirs)
	return parent, true
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
	fsPaths.GeneratedColimaConfigFilePath = filepath.Join(fsPaths.GeneratedArtifactsDir, "colima.yaml")

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
