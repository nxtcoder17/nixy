package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
)

type Context struct {
	context.Context

	*Env
	PWD        string
	AppVersion string

	nixyConfigHash string
	SystemOS       string
	SystemARCH     string

	// ColimaARCH overrides SystemARCH for platform resolution when the colima
	// VM runs a different architecture than the host.
	ColimaARCH string

	// DockerPlatform overrides the container platform when docker-mode.platform
	// is set.
	DockerPlatform string
}

func (c *Context) FlakePath() string {
	return filepath.Join(c.PWD, c.NixyProjectDir)
}

func NewContext(ctx context.Context, version string) (*Context, error) {
	e, err := LoadEnv()
	if err != nil {
		return nil, err
	}

	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return &Context{
		Context:    ctx,
		Env:        e,
		PWD:        pwd,
		AppVersion: version,
		SystemOS:   runtime.GOOS,
		SystemARCH: runtime.GOARCH,
	}, nil
}

func (c *Context) UpdateConfigHash(h string) {
	c.nixyConfigHash = h
}

func (c *Context) ConfigHash() string {
	return c.nixyConfigHash
}

// OSArch is the platform where the shell runs: Linux for non-local executors.
func (c *Context) OSArch() string {
	switch c.NixyMode {
	case DockerMode:
		if c.DockerPlatform != "" {
			return c.DockerPlatform
		}
	case ColimaMode:
		if c.ColimaARCH != "" {
			return "linux/" + c.ColimaARCH
		}
	case BubbleWrapMode:
	default:
		return c.SystemOS + "/" + c.SystemARCH
	}

	return "linux/" + c.SystemARCH
}
