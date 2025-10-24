package nixy

import (
	"reflect"
	"testing"
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
