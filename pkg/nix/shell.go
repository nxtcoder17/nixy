package nix

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func (n *Nix) Shell(ctx context.Context, shell string) error {
	if shell == "" {
		v, ok := os.LookupEnv("SHELL")
		if !ok {
			return fmt.Errorf("must specify a valid shell in case SHELL env-var is not defined")
		}
		shell = v
	}

	cmd := exec.CommandContext(ctx, os.Getenv("SHELL"))
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	if cmd.Env == nil {
		cmd.Env = make([]string, 0, 1)
	}

	paths, err := n.BinPaths(ctx)
	if err != nil {
		return err
	}

	cmd.Env = append(cmd.Env,
		fmt.Sprintf("PATH=%s", strings.Join(paths, ":")+":"+os.Getenv("PATH")),
		"IN_NIXY_SHELL=true",
	)

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
