package app

import "testing"

func TestContext_OSArch_ReturnsExecutorPlatform(t *testing.T) {
	tests := []struct {
		name       string
		mode       NixyMode
		os         string
		arch       string
		colimaArch string
		want       string
	}{
		{
			name: "local mode uses the host platform",
			mode: LocalMode,
			os:   "darwin",
			arch: "arm64",
			want: "darwin/arm64",
		},
		{
			name: "local-ignore-env mode uses the host platform",
			mode: LocalIgnoreEnvMode,
			os:   "linux",
			arch: "amd64",
			want: "linux/amd64",
		},
		{
			name: "docker mode uses the linux container platform",
			mode: DockerMode,
			os:   "darwin",
			arch: "arm64",
			want: "linux/arm64",
		},
		{
			name: "colima mode uses the linux VM platform",
			mode: ColimaMode,
			os:   "darwin",
			arch: "amd64",
			want: "linux/amd64",
		},
		{
			name:       "colima mode uses an explicit VM arch",
			mode:       ColimaMode,
			os:         "darwin",
			arch:       "arm64",
			colimaArch: "amd64",
			want:       "linux/amd64",
		},
		{
			name: "bubblewrap mode uses the linux sandbox platform",
			mode: BubbleWrapMode,
			os:   "linux",
			arch: "arm64",
			want: "linux/arm64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{
				Env:        &Env{NixyMode: tt.mode},
				SystemOS:   tt.os,
				SystemARCH: tt.arch,
				ColimaARCH: tt.colimaArch,
			}

			if got := ctx.OSArch(); got != tt.want {
				t.Errorf("OSArch() = %q, want %q", got, tt.want)
			}
		})
	}
}
