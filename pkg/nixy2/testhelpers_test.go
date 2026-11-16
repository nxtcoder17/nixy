package nixy2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const dockerStub = "#!/bin/sh\nexit 0\n"

// colimaStub reports a stopped VM and seeds $TEST_SSH_CONFIG on start.
const colimaStub = `#!/bin/sh
case "$1" in
status) exit 1 ;;
start) mkdir -p "$(dirname "$TEST_SSH_CONFIG")"; : > "$TEST_SSH_CONFIG" ;;
ssh) exit 0 ;;
esac
`

// stubs puts fake executables ahead of the system PATH, so the host's
// git/docker/colima never win. Returns the stub dir.
func stubs(t *testing.T, bins map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for name, body := range bins {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

// isolatedPath keeps only dir on PATH, so any tool without a stub reads as
// not installed.
func isolatedPath(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir)
}

// fakeGit returns a git stub serving `worktree list --porcelain`. Empty
// worktrees makes it fail the way git does outside a repository.
func fakeGit(worktrees string) string {
	if worktrees == "" {
		return "#!/bin/sh\necho 'fatal: not a git repository' >&2\nexit 128\n"
	}

	return "#!/bin/sh\ncat <<'EOF'\n" + worktrees + "EOF\n"
}

func porcelain(paths ...string) string {
	var b strings.Builder
	for _, path := range paths {
		b.WriteString("worktree " + path + "\nHEAD 1111111111111111111111111111111111111111\nbranch refs/heads/x\n\n")
	}

	return b.String()
}

func volumeMounts(args []string) []string {
	var mounts []string
	for i, arg := range args {
		if arg == "-v" && i+1 < len(args) {
			mounts = append(mounts, args[i+1])
		}
	}

	return mounts
}
