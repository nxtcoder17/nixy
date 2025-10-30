package nixy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

type Context struct {
	context.Context

	NixyProfile    string
	NixyMode       Mode
	NixyUseProfile bool
	NixyBinPath    string
	InNixyShell    bool

	PWD string

	// Nixy Constants
	NixyDataDir string
}

func (ctx *Context) IsLocalMode() bool {
	return ctx.NixyMode == LocalMode
}

func NewContext(parent context.Context, workspaceDir string) (*Context, error) {
	ctx := Context{
		Context: parent,
		PWD:     workspaceDir,
	}

	if v, ok := os.LookupEnv("NIXY_SHELL"); ok {
		ctx.InNixyShell = strings.EqualFold(v, "true")
	}

	if v, ok := os.LookupEnv("NIXY_PROFILE"); ok {
		ctx.NixyProfile = v
	} else {
		ctx.NixyProfile = "default"
	}

	if v, ok := os.LookupEnv("NIXY_EXECUTOR"); ok {
		ctx.NixyMode = Mode(v)
	} else {
		ctx.NixyMode = LocalMode
	}

	if v, ok := os.LookupEnv("NIXY_USE_PROFILE"); ok {
		v = strings.TrimSpace(v)
		ctx.NixyUseProfile = v == "1" || strings.EqualFold(v, "true")
	}

	var err error
	ctx.NixyBinPath, err = getCallerBinPath()
	if err != nil {
		return nil, err
	}

	if v, ok := os.LookupEnv("NIXY_SHELL"); ok {
		ctx.InNixyShell = strings.EqualFold(v, "true")
	}

	return &ctx, nil
}

func getCallerBinPath() (string, error) {
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
}
