package nixy2

import (
	"github.com/nxtcoder17/nixy/internal/app"
	"os/exec"
)

type localExecutor struct {
	NixStoreDir string
	HomeDir     string

	Mounts []NixyMount
	Env    map[string]string
}

func (n *Nixy) NewLocalExecutor(appCtx *app.Context, fsPaths *FSPaths) Executor {
	return &localExecutor{
		NixStoreDir: fsPaths.NixStoreDir,
		HomeDir:     fsPaths.UserHomeDir,
		Mounts:      n.Mounts,
		Env:         n.Env,
	}
}

func (d *localExecutor) Exec(appCtx *app.Context, cmd string, args ...string) (*exec.Cmd, error) {
	return exec.CommandContext(appCtx.Context, cmd, args...), nil
}
