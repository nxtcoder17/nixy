package nixy2

import (
	"context"
	"slices"
	"testing"

	"github.com/nxtcoder17/nixy/internal/app"
)

func newDockerExecArgs(t *testing.T, pwd string) []string {
	t.Helper()

	appCtx := &app.Context{
		Context: context.Background(),
		Env: &app.Env{
			NixyMode:              app.DockerMode,
			NixyDockerModeImage:   "nixy-test",
			NixyExecutableBinPath: "/bin/nixy",
		},
		PWD: pwd,
	}
	n := &Nixy{NixyYAML: &NixyYAML{}}

	cmd, err := n.NewDockerExecutor(appCtx, &FSPaths{}).Exec(appCtx, "nix", "--version")
	if err != nil {
		t.Fatal(err)
	}

	return cmd.Args
}

func TestDockerExecutor_WorktreeMounts(t *testing.T) {
	tests := []struct {
		name      string
		worktrees string
		noGit     bool
		pwd       string
		wantExtra []string
	}{
		{
			name: "not a git repo adds no mount",
			pwd:  "/work/repo",
		},
		{
			name:  "git missing adds no mount",
			noGit: true,
			pwd:   "/work/repo",
		},
		{
			name:      "no worktrees adds no mount",
			worktrees: porcelain("/work/repo"),
			pwd:       "/work/repo",
		},
		{
			name:      "shared parent mounts once",
			worktrees: porcelain("/work/repo", "/work/trees/one", "/work/trees/two"),
			pwd:       "/work/repo",
			wantExtra: []string{"/work:/work:z"},
		},
		{
			name:      "worktree under pwd adds no mount",
			worktrees: porcelain("/work/repo", "/work/repo/trees/one"),
			pwd:       "/work/repo",
		},
		{
			name:      "bare repo with external worktree adds no mount",
			worktrees: porcelain("/repos/foo.git", "/work/foo/master"),
			pwd:       "/work/foo/master",
		},
		{
			name:      "common parent is home adds no mount",
			worktrees: porcelain("/home/dev/a", "/home/dev/b"),
			pwd:       "/home/dev/a",
		},
		{
			name:      "bare repo with internal worktree mounts repo dir",
			worktrees: porcelain("/srv/foo.git", "/srv/foo.git/master"),
			pwd:       "/srv/foo.git/master",
			wantExtra: []string{"/srv/foo.git:/srv/foo.git:z"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", "/home/dev")

			bins := map[string]string{"docker": dockerStub}
			if !tt.noGit {
				bins["git"] = fakeGit(tt.worktrees)
			}
			dir := stubs(t, bins)
			if tt.noGit {
				isolatedPath(t, dir)
			}

			got := volumeMounts(newDockerExecArgs(t, tt.pwd))

			want := append([]string{
				"nixy-nix-store:/nix:z",
				"nixy-user-home:/home:z",
				tt.pwd + ":" + tt.pwd + ":z",
				tt.pwd + ":/workspace:z",
			}, tt.wantExtra...)
			want = append(want, "/bin/nixy:/bin/nixy:ro,z")

			if !slices.Equal(got, want) {
				t.Fatalf("mounts = %v, want %v", got, want)
			}
		})
	}
}

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
			stubs(t, map[string]string{"docker": dockerStub})

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
