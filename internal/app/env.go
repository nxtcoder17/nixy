package app

import (
	"os"
	"path/filepath"
	"reflect"

	"github.com/codingconcepts/env"
)

type NixyMode string

func (nm NixyMode) Str() string {
	return string(nm)
}

const (
	LocalMode          NixyMode = "local"
	LocalIgnoreEnvMode NixyMode = "local-ignore-env"
	DockerMode         NixyMode = "docker"
	BubbleWrapMode     NixyMode = "bubblewrap"
	ColimaMode         NixyMode = "colima"
)

type Env struct {
	InNixyShell bool `env:"NIXY_SHELL" default:"false" usage:"set by nixy inside a nixy shell"`

	NixyMode NixyMode `env:"NIXY_MODE" default:"local" usage:"execution backend: local, local-ignore-env, docker, bubblewrap or colima"`

	NixyPreload string `env:"NIXY_PRELOAD" usage:"additional nixy.yml merged before the project's nixy.yml"`

	NixyProjectDir string `env:"NIXY_PROJECT_DIR" default:".nixy" usage:"dir next to nixy.yml holding nixy's generated files, inspect it to see what is part of your shell"`

	NixyGlobalDir string `env:"NIXY_GLOBAL_DIR" usage:"dir for nixy's global files (default: $XDG_DATA_HOME/nixy, else ~/.local/share/nixy)"`

	// NixyExecutableBinPath is resolved by nixy itself, so it is not configurable.
	NixyExecutableBinPath string `env:"-"`

	NixyDisableWorktreeMounts bool `env:"NIXY_DISABLE_WORKTREE_MOUNTS" default:"false" usage:"true keeps sibling git worktrees out of non-local shells"`

	NixyDockerModeImage string `env:"NIXY_DOCKER_MODE_IMAGE" default:"ghcr.io/nxtcoder17/nix:latest" usage:"image used by docker mode"`
}

// EnvVar describes an environment variable nixy reads.
type EnvVar struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Default string `json:"default,omitempty"`
	Usage   string `json:"usage"`
}

// EnvVars lists the environment variables nixy reads, in declaration order,
// with the values seen in the current environment.
func EnvVars() []EnvVar {
	t := reflect.TypeOf(Env{})

	vars := make([]EnvVar, 0, t.NumField())
	for i := range t.NumField() {
		field := t.Field(i)

		name := field.Tag.Get("env")
		if name == "" || name == "-" {
			continue
		}

		vars = append(vars, EnvVar{
			Name:    name,
			Value:   os.Getenv(name),
			Default: field.Tag.Get("default"),
			Usage:   field.Tag.Get("usage"),
		})
	}

	return vars
}

func LoadEnv() (*Env, error) {
	e := &Env{}
	if err := env.Set(e); err != nil {
		return nil, err
	}

	if e.NixyGlobalDir == "" {
		e.NixyGlobalDir = defaultGlobalDir()
	}

	exe, err := executablePath()
	if err != nil {
		return nil, err
	}
	e.NixyExecutableBinPath = exe

	return e, nil
}

func defaultGlobalDir() string {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}

	return filepath.Join(dir, "nixy")
}

func executablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	return filepath.EvalSymlinks(exe)
}
