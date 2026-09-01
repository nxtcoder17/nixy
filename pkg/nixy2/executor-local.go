package nixy2

import (
	"github.com/nxtcoder17/nixy/internal/app"
	fn "github.com/nxtcoder17/nixy/pkg/functions"
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
		Env:         fn.MapMerge(n.Env, commonExecutorEnv(appCtx)),
	}
}

func (l *localExecutor) Exec(appCtx *app.Context, cmd string, args ...string) (*exec.Cmd, error) {
	// INFO: Nixy is a developer CLI tool that inherently runs commands defined by the user.
	// The commands and args passed here are constructed by the Nixy tool itself.
	// #nosec G204
	c := exec.CommandContext(appCtx.Context, cmd, args...)
	c.Env = append(c.Env, fn.ToEnviron(l.Env)...)
	return c, nil
}
