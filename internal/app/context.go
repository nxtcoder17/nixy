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

func (c *Context) OSArch() string {
	return c.SystemOS + "/" + c.SystemARCH
}
