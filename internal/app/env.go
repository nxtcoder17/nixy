package app

import (
	"github.com/codingconcepts/env"
	"os"
	"path/filepath"
)

type NixyMode string

const (
	LocalMode          NixyMode = "local"
	LocalIgnoreEnvMode NixyMode = "local-ignore-env"
	DockerMode         NixyMode = "docker"
	BubbleWrapMode     NixyMode = "bubblewrap"
	ColimaMode         NixyMode = "colima"
)

type Env struct {
	// InNixyShell is just a check to determine whether nixy is running in a `nixy shell`
	InNixyShell bool `env:"NIXY_SHELL" default:"false"`

	// NixyMode controls Nixy's execution mode
	NixyMode NixyMode `env:"NIXY_MODE" default:"local"`

	// NixyPreload is totally separate nixy.yml file, that is used while preparing nixy shell's environment
	NixyPreload string `env:"NIXY_PRELOAD"`

	// NixyProjectDir is directory inside a user's project next to `nixy.yml` file, in while nixy puts it's inflight and computed files
	// This directory can be used to inspect, what's exactly is a part of your nixy shell
	NixyProjectDir string `env:"NIXY_PROJECT_DIR" default:".nixy"`

	// NixyGlobalDir is directory where nixy puts global files.
	// It defaults to $XDG_DATA_HOME/nixy.
	// like,
	// - nixy shell user's home directory
	NixyGlobalDir string `env:"NIXY_GLOBAL_DIR"`

	// NixyExecutableBinPath is absolute path to the nixy executable, it is used to ensure nixy binary is in nixy shell's PATH
	NixyExecutableBinPath string `env:"-"`

	NixyDockerModeImage string `env:"NIXY_DOCKER_MODE_IMAGE" default:"ghcr.io/nxtcoder17/nix:nixy"`
}

func LoadEnv() (*Env, error) {
	e := &Env{}
	if err := env.Set(e); err != nil {
		return nil, err
	}

	e.NixyGlobalDir = func() string {
		d := os.Getenv("XDG_DATA_HOME")
		if d == "" {
			d = filepath.Join(os.Getenv("HOME"), ".local", "share")
		}

		return filepath.Join(d, "nixy")
	}()

	var ferr error
	e.NixyExecutableBinPath, ferr = func() (string, error) {
		exe, err := os.Executable()
		if err != nil {
			return "", err
		}

		// Resolve symlinks and get absolute path
		exePath, err := filepath.EvalSymlinks(exe)
		if err != nil {
			return "", err
		}

		return exePath, nil
	}()

	if ferr != nil {
		return nil, ferr
	}

	return e, nil
}
