package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/nxtcoder17/fastlog"
	errors "github.com/nxtcoder17/go.errors"
	"github.com/nxtcoder17/nixy/internal/app"
	"github.com/nxtcoder17/nixy/pkg/nixy2"
	"github.com/nxtcoder17/nixy/pkg/nixy2/templates"
	"github.com/urfave/cli/v3"
)

var Version string

//go:embed shell/hook.fish
var shellHookFish string

//go:embed shell/hook.bash
var shellHookBash string

//go:embed shell/hook.zsh
var shellHookZsh string

func main() {
	if Version == "" {
		Version = fmt.Sprintf("nightly | %s", time.Now().Format(time.RFC3339))
	}

	ctx, cf := signal.NotifyContext(context.TODO(), syscall.SIGINT, syscall.SIGTERM)
	defer cf()

	cmd := cli.Command{
		Name:        "nixy",
		Version:     Version,
		Description: "An approachable nix based development workspace setup tool",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:     "debug",
				Usage:    "--debug",
				Required: false,
				Value:    false,
			},
		},

		// ShellCompletionCommandName: "completion:shell",
		EnableShellCompletion: true,

		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			logger := fastlog.New().DebugMode(c.Bool("debug")).SkipCallerFrames(1).Console()
			fastlog.SetDefaultLogger(logger)
			return ctx, nil
		},

		Commands: []*cli.Command{
			actionInit(),
			actionShellHook(),
			actionShell(),
			actionBuild(),
			actionColima(),
		},

		Suggest: true,
	}

	go func() {
		<-ctx.Done()
		cf()
	}()

	if err := cmd.Run(ctx, os.Args); err != nil {
		fastlog.Error(err.Error())
	}
}

func locateNearestNixyFile() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	oldDir := ""
	nixyConfigFiles := []string{"nixy.yml"}

	for oldDir != dir {
		for _, fn := range nixyConfigFiles {
			if _, err := os.Stat(filepath.Join(dir, fn)); err != nil {
				if !os.IsNotExist(err) {
					return "", err
				}
				continue
			}

			return filepath.Join(dir, fn), nil
		}

		oldDir = dir
		dir = filepath.Dir(dir)
	}

	return "", errors.New("failed to locate your nearest Nixyfile")
}

func loadFromNixyfile(appCtx *app.Context, c *cli.Command) (*nixy2.Nixy, error) {
	if c.IsSet("file") {
		return nixy2.LoadFromFile(appCtx, c.String("file"))
	}

	f, err := locateNearestNixyFile()
	if err != nil {
		return nil, err
	}

	return nixy2.LoadFromFile(appCtx, f)
}

func writeDockerfile(projectDir string, build nixy2.Build) error {
	b, err := templates.RenderDockerfile()
	if err != nil {
		return err
	}

	dockerfilePath := filepath.Join(build.BuildDir(projectDir), "Dockerfile")
	if _, err := os.Stat(dockerfilePath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return errors.New("failed to check dockerfile path").Wrap(err)
	}

	if err := os.WriteFile(dockerfilePath, b, 0o644); err != nil {
		return errors.New("failed to write dockerfile path").Wrap(err)
	}

	return nil
}

func actionInit() *cli.Command {
	return &cli.Command{
		Name:    "init",
		Suggest: true,
		Action: func(ctx context.Context, _ *cli.Command) error {
			return nixy2.CreateNixyYAML(ctx)
		},
	}
}

func actionShellHook() *cli.Command {
	return &cli.Command{
		Name:    "shell:hook",
		Suggest: true,
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:   "shell",
				Config: cli.StringConfig{TrimSpace: true},
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			shell := c.StringArg("shell")
			switch shell {
			case "fish":
				fmt.Print(shellHookFish)
			case "bash":
				fmt.Print(shellHookBash)
			case "zsh":
				fmt.Print(shellHookZsh)
			default:
				return fmt.Errorf("unsupported shell: %s (supported: fish, bash, zsh)", shell)
			}
			return nil
		},
	}
}

func actionShell() *cli.Command {
	return &cli.Command{
		Name:    "shell",
		Suggest: true,
		Action: func(ctx context.Context, c *cli.Command) error {
			appCtx, err := app.NewContext(ctx, Version)
			if err != nil {
				return err
			}

			n, err := loadFromNixyfile(appCtx, c)
			if err != nil {
				return err
			}

			if err := n.Shell(appCtx, strings.Join(c.Args().Slice(), " ")); err != nil {
				return err
			}

			return nil
		},
	}
}

func actionBuild() *cli.Command {
	return &cli.Command{
		Name:    "build",
		Suggest: true,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "dockerfile",
				Usage: "generate a Dockerfile for the selected build target, that consumes the created build artifact",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			appCtx, err := app.NewContext(ctx, Version)
			if err != nil {
				return err
			}

			if appCtx.Env.InNixyShell {
				n, err := nixy2.LoadInNixyShell(appCtx)
				if err != nil {
					return err
				}

				for _, target := range c.Args().Slice() {
					if err := n.Build(appCtx, target); err != nil {
						return err
					}

					if c.Bool("dockerfile") {
						return writeDockerfile(appCtx.PWD, n.Builds[target])
					}
				}

				return nil
			}

			n, err := loadFromNixyfile(appCtx, c)
			if err != nil {
				return err
			}

			for _, target := range c.Args().Slice() {
				if err := n.Build(appCtx, target); err != nil {
					return err
				}

				if c.Bool("dockerfile") {
					return writeDockerfile(appCtx.PWD, n.Builds[target])
				}
			}

			return nil
		},
	}
}

func actionColima() *cli.Command {
	return &cli.Command{
		Name:    "colima",
		Suggest: true,
		Commands: []*cli.Command{
			actionColimaStop(),
			actionColimaSSH(),
		},
	}
}

func actionColimaStop() *cli.Command {
	return &cli.Command{
		Name:  "stop",
		Usage: "stop the Colima VM associated with the project",
		Action: func(ctx context.Context, c *cli.Command) error {
			appCtx, err := app.NewContext(ctx, Version)
			if err != nil {
				return err
			}

			n, err := loadFromNixyfile(appCtx, c)
			if err != nil {
				n = &nixy2.Nixy{}
			}

			fsPaths, err := nixy2.CreateFSPaths(appCtx)
			if err != nil {
				return err
			}

			executor := n.NewColimaExecutor(appCtx, fsPaths)
			if colimaExec, ok := executor.(interface {
				Stop(appCtx *app.Context) error
			}); ok {
				return colimaExec.Stop(appCtx)
			}

			return fmt.Errorf("current execution mode is not Colima")
		},
	}
}

func actionColimaSSH() *cli.Command {
	return &cli.Command{
		Name:  "ssh",
		Usage: "SSH into the Colima VM associated with the project",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "raw",
				Usage: "print the raw SSH command to stdout instead of running it",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			appCtx, err := app.NewContext(ctx, Version)
			if err != nil {
				return err
			}

			n, err := loadFromNixyfile(appCtx, c)
			if err != nil {
				n = &nixy2.Nixy{}
			}

			fsPaths, err := nixy2.CreateFSPaths(appCtx)
			if err != nil {
				return err
			}

			executor := n.NewColimaExecutor(appCtx, fsPaths)
			if colimaExec, ok := executor.(interface {
				SSH(appCtx *app.Context, raw bool) error
			}); ok {
				return colimaExec.SSH(appCtx, c.Bool("raw"))
			}

			return fmt.Errorf("current execution mode is not Colima")
		},
	}
}
