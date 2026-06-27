package nixy2

import (
	"github.com/nxtcoder17/nixy/internal/app"
	"os/exec"
)

type Executor interface {
	Exec(appCtx *app.Context, command string, args ...string) (*exec.Cmd, error)
}
