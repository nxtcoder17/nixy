package nixy2

import (
	"github.com/nxtcoder17/nixy/internal/app"
	"os/exec"
)

type Executor interface {
	Exec(appCtx *app.Context, command string, args ...string) (*exec.Cmd, error)
}

func commonExecutorEnv(appCtx *app.Context) map[string]string {
	return map[string]string{
		"NIXY_MODE":  appCtx.NixyMode.Str(),
		"NIXY_SHELL": "true",
	}
}
