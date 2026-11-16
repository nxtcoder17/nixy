package main

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/nxtcoder17/nixy/internal/app"
)

// sampleEnvVars covers a set var, an unset var and a var without a default.
func sampleEnvVars() []app.EnvVar {
	return []app.EnvVar{
		{Name: "NIXY_MODE", Default: "local", Value: "docker", Usage: "backend"},
		{Name: "NIXY_PRELOAD", Usage: "preload"},
	}
}

func TestWriteEnvVars_Table_RendersHeaderAndRows(t *testing.T) {
	var buf bytes.Buffer
	if err := writeEnvVars(&buf, sampleEnvVars(), false); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3: %q", len(lines), buf.String())
	}

	tests := []struct {
		name string
		line int
		want []string
	}{
		{name: "header", line: 0, want: []string{"NAME", "DEFAULT", "CURRENT", "USAGE"}},
		{name: "set var", line: 1, want: []string{"NIXY_MODE", "local", "docker", "backend"}},
		{name: "unset var", line: 2, want: []string{"NIXY_PRELOAD", "-", "-", "preload"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strings.Fields(lines[tt.line]); !slices.Equal(got, tt.want) {
				t.Errorf("line %d = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestWriteEnvVars_JSON_MatchesVars(t *testing.T) {
	want := sampleEnvVars()

	var buf bytes.Buffer
	if err := writeEnvVars(&buf, want, true); err != nil {
		t.Fatal(err)
	}

	var got []app.EnvVar
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if !slices.Equal(got, want) {
		t.Fatalf("json = %+v, want %+v", got, want)
	}
}
