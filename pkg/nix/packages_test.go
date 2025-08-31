package nix

import (
	"reflect"
	"testing"
)

func Test_parsePackage(t *testing.T) {
	tests := []struct {
		name    string
		pkg     any
		want    *NormalizedPackage
		wantErr bool
	}{
		{
			name: "[VALID] simple package reference",
			pkg:  "go",
			want: &NormalizedPackage{
				IsNixPackage: true,
				Name:         "go",
				Commit:       "",
			},
			wantErr: false,
		},
		{
			name: "[VALID] pinned nixpkgs package",
			pkg:  "nixpkgs/41d292bfc37309790f70f4c120b79280ce40af16#go",
			want: &NormalizedPackage{
				IsNixPackage: true,
				Name:         "go",
				Commit:       "41d292bfc37309790f70f4c120b79280ce40af16",
			},
			wantErr: false,
		},
		{
			name:    "[INVALID] pinned nixpkgs package",
			pkg:     "nixpkgs/41d292bfc37309790f70f4c120b79280ce40af16/go",
			want:    nil,
			wantErr: true,
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			np, err := parsePackage(tt.pkg)
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
