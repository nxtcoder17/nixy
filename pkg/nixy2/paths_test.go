package nixy2

import (
	"context"
	"path/filepath"
	"testing"
)

func TestParseGitWorktreeList_WorktreesAndMetadata_ReturnsWorktreePaths(t *testing.T) {
	out := []byte(`worktree /Users/x/nixy/master
HEAD 1111111111111111111111111111111111111111
branch refs/heads/master

worktree /Users/x/nixy/worktrees/feature
HEAD 2222222222222222222222222222222222222222
branch refs/heads/feature
`)

	got := parseGitWorktreeList(out)
	want := []string{"/Users/x/nixy/master", "/Users/x/nixy/worktrees/feature"}

	if len(got) != len(want) {
		t.Fatalf("parseGitWorktreeList() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("parseGitWorktreeList()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseGitWorktreeList_EmptyWorktreeEntry_Skipped(t *testing.T) {
	if got := parseGitWorktreeList([]byte("worktree \nHEAD 1111\n")); len(got) != 0 {
		t.Fatalf("parseGitWorktreeList() = %v, want none", got)
	}
}

func TestCommonDirPrefix(t *testing.T) {
	tests := []struct {
		name string
		dirs []string
		want string
	}{
		{
			name: "single dir returns itself",
			dirs: []string{filepath.Join("/a", "repo")},
			want: filepath.Join("/a", "repo"),
		},
		{
			name: "siblings return common parent",
			dirs: []string{filepath.Join("/a", "repo"), filepath.Join("/a", "worktrees", "one")},
			want: "/a",
		},
		{
			name: "nested dir returns outer dir",
			dirs: []string{filepath.Join("/a", "repo"), filepath.Join("/a", "repo", "wt")},
			want: filepath.Join("/a", "repo"),
		},
		{
			name: "unrelated roots return root",
			dirs: []string{"/a", "/b"},
			want: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := commonDirPrefix(tt.dirs); got != tt.want {
				t.Fatalf("commonDirPrefix(%v) = %q, want %q", tt.dirs, got, tt.want)
			}
		})
	}
}

func TestGetGitWorktreesParentDir(t *testing.T) {
	tests := []struct {
		name      string
		worktrees string
		noGit     bool
		want      string
		wantOK    bool
	}{
		{name: "not a git repo returns false"},
		{name: "git missing returns false", noGit: true},
		{name: "single worktree returns false", worktrees: porcelain("/work/repo")},
		{
			name:      "sibling worktrees return shared parent",
			worktrees: porcelain("/work/repo", "/work/trees/one"),
			want:      "/work",
			wantOK:    true,
		},
		{
			name:      "nested worktree returns project dir",
			worktrees: porcelain("/work/repo", "/work/repo/trees/one"),
			want:      "/work/repo",
			wantOK:    true,
		},
		{
			name:      "bare repo with external worktree returns false",
			worktrees: porcelain("/repos/foo.git", "/work/foo/master"),
		},
		{
			name:      "common parent is home returns false",
			worktrees: porcelain("/home/dev/a", "/home/dev/b"),
		},
		{
			name:      "bare repo with internal worktree returns repo dir",
			worktrees: porcelain("/srv/foo.git", "/srv/foo.git/master"),
			want:      "/srv/foo.git",
			wantOK:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", "/home/dev")

			bins := map[string]string{}
			if !tt.noGit {
				bins["git"] = fakeGit(tt.worktrees)
			}
			dir := stubs(t, bins)
			if tt.noGit {
				isolatedPath(t, dir)
			}

			got, ok := GetGitWorktreesParentDir(context.Background(), "/work/repo")
			if ok != tt.wantOK || got != tt.want {
				t.Fatalf("GetGitWorktreesParentDir() = %q, %v, want %q, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestIsSafeMountRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tests := []struct {
		name string
		root string
		want bool
	}{
		{name: "root dir is unsafe", root: "/", want: false},
		{name: "empty is unsafe", root: "", want: false},
		{name: "home is unsafe", root: home, want: false},
		{name: "home parent is unsafe", root: filepath.Dir(home), want: false},
		{name: "project parent is safe", root: filepath.Join("/tmp", "projects"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSafeMountRoot(tt.root); got != tt.want {
				t.Fatalf("isSafeMountRoot(%q) = %v, want %v", tt.root, got, tt.want)
			}
		})
	}
}
