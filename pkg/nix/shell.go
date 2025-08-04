package nix

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
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

	paths, err := n.BinPaths(ctx)
	if err != nil {
		return err
	}

	envOverride := []string{
		"IN_NIXY_SHELL=true",
	}

	var args []string

	switch path.Base(shell) {
	case "bash":
		{
			f, err := os.CreateTemp("", ".sh")
			if err != nil {
				return err
			}
			defer func() {
				f.Close()
				os.Remove(f.Name())
			}()

			fmt.Fprintln(f, `[ -e  "~/.bashrc" ] && source ~/.bashrc`)
			fmt.Fprintf(f, "export PATH=%s:$PATH\n", strings.Join(paths, ":"))

			args = append(args, "--rcfile", f.Name())
		}
	case "zsh":
		{
			dir, err := os.MkdirTemp("", "zsh")
			if err != nil {
				return err
			}

			if err := os.WriteFile(filepath.Join(dir, ".zshrc"), []byte(strings.Join([]string{
				fmt.Sprintf("export ZDOTDIR=%s", func() string {
					if v, ok := os.LookupEnv("ZDOTDIR"); ok {
						return v
					}
					return "$HOME"
				}()),
				`[ -e "$ZDOTDIR/.zshrc" ] && source "$ZDOTDIR/.zshrc"`,
				fmt.Sprintf("export PATH=%s:$PATH\n", strings.Join(paths, ":")),
			}, "\n")), 0o644); err != nil {
				return err
			}
			defer func() {
				os.RemoveAll(dir)
			}()

			envOverride = append(envOverride, "ZDOTDIR="+dir)
		}
	case "fish":
		{
			f, err := os.CreateTemp("", ".sh")
			if err != nil {
				return err
			}

			defer func() {
				f.Close()
				os.Remove(f.Name())
			}()

			fmt.Fprintln(f, `[ -e "~/.config/fish/config.fish" ] && source ~/.config/fish/config.fish`)
			fmt.Fprintf(f, "fish_add_path %s\n", strings.Join(paths, " "))

			args = append(args, "--init-command", "source "+f.Name())
		}
	}

	cmd := exec.CommandContext(ctx, os.Getenv("SHELL"), args...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	cmd.Env = append(os.Environ(),
		envOverride...,
	)

	slog.Debug("Executing", "command", cmd.String())

	defer func() {
		slog.Debug("Shell Exited")
	}()

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
