package nixy

import (
	"bytes"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func Test_parsePackage(t *testing.T) {
	tests := []struct {
		name    string
		pkg     string
		want    *NormalizedPackage
		wantErr bool
	}{
		{
			name: "[VALID] simple package reference",
			pkg:  "go",
			want: &NormalizedPackage{
				NixPackage: &NixPackage{
					Name:   "go",
					Commit: "",
				},
			},
			wantErr: false,
		},
		{
			name: "[VALID] package with nixpkgs key",
			pkg:  "stable#go",
			want: &NormalizedPackage{
				NixPackage: &NixPackage{
					Name:   "go",
					Commit: "stable",
				},
			},
			wantErr: false,
		},
		{
			name: "[VALID] package with custom key",
			pkg:  "unstable#python314",
			want: &NormalizedPackage{
				NixPackage: &NixPackage{
					Name:   "python314",
					Commit: "unstable",
				},
			},
			wantErr: false,
		},
		{
			name: "[VALID] package with nested attribute",
			pkg:  "cuda#cudaPackages.cudatoolkit",
			want: &NormalizedPackage{
				NixPackage: &NixPackage{
					Name:   "cudaPackages.cudatoolkit",
					Commit: "cuda",
				},
			},
			wantErr: false,
		},

		// {
		// 	name: "[VALID] url package",
		// 	pkg: map[string]any{
		// 		"name": "sample",
		// 		"url":  "https://sample.go/download",
		// 	},
		// 	want: &NormalizedPackage{
		// 		URLPackage: &URLPackage{
		// 			Name:   "sample",
		// 			URL:    "https://sample.go/download",
		// 			Sha256: "",
		// 		},
		// 	},
		// 	wantErr: false,
		// },
		// {
		// 	name: "[VALID] url package with SHA256",
		// 	pkg: map[string]any{
		// 		"name":   "sample",
		// 		"url":    "https://sample.go/download",
		// 		"sha256": "SAMPLE-SHA",
		// 	},
		// 	want: &NormalizedPackage{
		// 		URLPackage: &URLPackage{
		// 			Name:   "sample",
		// 			URL:    "https://sample.go/download",
		// 			Sha256: "SAMPLE-SHA",
		// 		},
		// 	},
		// 	wantErr: false,
		// },
		//
		// {
		// 	name: "[INVALID] url package without a name",
		// 	pkg: map[string]any{
		// 		"name":   "",
		// 		"url":    "https://sample.go/download",
		// 		"sha256": "SAMPLE-SHA",
		// 	},
		// 	want:    nil,
		// 	wantErr: true,
		// },
		// {
		// 	name: "[INVALID] url package without a url",
		// 	pkg: map[string]any{
		// 		"name": "sample",
		// 		"url":  "",
		// 	},
		// 	want:    nil,
		// 	wantErr: true,
		// },
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			np, err := parseNixPackage(tt.pkg)
			if tt.wantErr && err == nil {
				t.Errorf("wanted error, but got no error")
			}
			if err != nil && !tt.wantErr {
				t.Errorf("Assertion Failed | \n\tgot: %v\n\texpected: nil", err)
				return
			}

			if !reflect.DeepEqual(np, tt.want) {
				t.Errorf("Assertion Failed \n\tgot: %v\n\texpected: %v", np, tt.want)
			}
		})
	}
}

func TestURLPackage_MarshalYAML_KeyOrdering(t *testing.T) {
	tests := []struct {
		name string
		pkg  *NormalizedPackage
		want string
	}{
		{
			name: "single platform",
			pkg: &NormalizedPackage{
				URLPackage: &URLPackage{
					Name: "run",
					Sources: map[string]URLAndSHA{
						"linux/amd64": {URL: "https://example.com/run-linux-amd64", SHA256: "abc123"},
					},
					BinPaths:    []string{"bin/run"},
					InstallHook: "echo hello",
				},
			},
			want: `name: run
sources:
  linux/amd64:
    url: https://example.com/run-linux-amd64
    sha256: abc123
binPaths:
  - bin/run
installHook: |-
  echo hello
`,
		},
		{
			name: "multiple platforms sorted alphabetically",
			pkg: &NormalizedPackage{
				URLPackage: &URLPackage{
					Name: "run",
					Sources: map[string]URLAndSHA{
						"linux/amd64":  {URL: "https://example.com/run-linux-amd64", SHA256: "linux123"},
						"darwin/arm64": {URL: "https://example.com/run-darwin-arm64", SHA256: "darwin123"},
						"darwin/amd64": {URL: "https://example.com/run-darwin-amd64", SHA256: "darwinx86"},
					},
				},
			},
			want: `name: run
sources:
  darwin/amd64:
    url: https://example.com/run-darwin-amd64
    sha256: darwinx86
  darwin/arm64:
    url: https://example.com/run-darwin-arm64
    sha256: darwin123
  linux/amd64:
    url: https://example.com/run-linux-amd64
    sha256: linux123
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			encoder := yaml.NewEncoder(&buf)
			encoder.SetIndent(2)
			if err := encoder.Encode(tt.pkg); err != nil {
				t.Fatalf("failed to encode: %v", err)
			}
			got := buf.String()
			if got != tt.want {
				t.Errorf("mismatch:\ngot:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestUpdateSHA256InNode_PreservesComments(t *testing.T) {
	input := `# This is a top comment
nixpkgs:
  # default nixpkgs version
  default: "abc123"
packages:
  - go # Go compiler
  # URL package for run tool
  - name: run
    sources:
      linux/amd64:
        url: "https://example.com/run"
        sha256: ""
# Shell hooks
onShellEnter: |
  export PATH="$PWD/bin:$PATH"
`

	var rootNode yaml.Node
	if err := yaml.Unmarshal([]byte(input), &rootNode); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if err := updateSHA256InNode(&rootNode, 1, "linux/amd64", "newhash123"); err != nil {
		t.Fatalf("failed to update sha256: %v", err)
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&rootNode); err != nil {
		t.Fatalf("failed to encode: %v", err)
	}

	got := buf.String()
	want := `# This is a top comment
nixpkgs:
  # default nixpkgs version
  default: "abc123"
packages:
  - go # Go compiler
  # URL package for run tool
  - name: run
    sources:
      linux/amd64:
        url: "https://example.com/run"
        sha256: "newhash123"
# Shell hooks
onShellEnter: |
  export PATH="$PWD/bin:$PATH"
`

	if got != want {
		t.Errorf("mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestNixPackage_MarshalYAML(t *testing.T) {
	tests := []struct {
		name string
		pkg  *NormalizedPackage
		want string
	}{
		{
			name: "simple package",
			pkg:  &NormalizedPackage{NixPackage: &NixPackage{Name: "go"}},
			want: "go\n",
		},
		{
			name: "package with commit",
			pkg:  &NormalizedPackage{NixPackage: &NixPackage{Name: "go", Commit: "unstable"}},
			want: "nixpkgs/unstable#go\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			encoder := yaml.NewEncoder(&buf)
			if err := encoder.Encode(tt.pkg); err != nil {
				t.Fatalf("failed to encode: %v", err)
			}
			got := buf.String()
			if got != tt.want {
				t.Errorf("mismatch:\ngot:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}
