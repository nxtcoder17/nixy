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
	"github.com/nxtcoder17/nixy/pkg/nixy"
	"github.com/nxtcoder17/nixy/pkg/nixy/templates"
	"github.com/nxtcoder17/nixy/pkg/nixy2"
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

	appCtx, err := app.NewContext(ctx, Version)
	if err != nil {
		panic(err)
	}

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
			logger := fastlog.New().DebugMode(c.Bool("debug")).Console()
			fastlog.SetDefaultLogger(logger)
			return ctx, nil
		},

		Commands: []*cli.Command{
			actionInit(appCtx),
			actionShellHook(),
			actionShell(appCtx),
			actionBuild(appCtx),
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

func loadFromNixyfile(ctx context.Context, c *cli.Command) (*nixy.NixyWrapper, error) {
	if c.IsSet("file") {
		return nixy.LoadFromFile(ctx, c.String("file"))
	}

	f, err := locateNearestNixyFile()
	if err != nil {
		return nil, err
	}

	return nixy.LoadFromFile(ctx, f)
}

func writeDockerfile(projectDir string, build nixy.Build) error {
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

func actionInit(appCtx *app.Context) *cli.Command {
	return &cli.Command{
		Name:    "init",
		Suggest: true,
		Action: func(ctx context.Context, _ *cli.Command) error {
			if err := nixy2.CreateFSPaths(appCtx); err != nil {
				return err
			}
			return nixy.InitWorkspace(ctx)
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

func actionShell(appCtx *app.Context) *cli.Command {
	return &cli.Command{
		Name:    "shell",
		Suggest: true,
		Action: func(ctx context.Context, c *cli.Command) error {
			if err := nixy2.CreateFSPaths(appCtx); err != nil {
				return err
			}

			n, err := loadFromNixyfile(ctx, c)
			if err != nil {
				return err
			}

			if err := n.Shell(n.Context, strings.Join(c.Args().Slice(), " ")); err != nil {
				return err
			}

			return nil
		},
	}
}

func actionBuild(appCtx *app.Context) *cli.Command {
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
			if appCtx.Env.InNixyShell {
				n, err := nixy.LoadInNixyShell(ctx)
				if err != nil {
					return err
				}

				for _, target := range c.Args().Slice() {
					if err := n.Build(ctx, target); err != nil {
						return err
					}

					if c.Bool("dockerfile") {
						return writeDockerfile(n.PWD, n.Builds[target])
					}
				}

				return nil
			}

			if err := nixy2.CreateFSPaths(appCtx); err != nil {
				return err
			}

			n, err := loadFromNixyfile(ctx, c)
			if err != nil {
				return err
			}

			for _, target := range c.Args().Slice() {
				if err := n.Build(n.Context, target); err != nil {
					return err
				}

				if c.Bool("dockerfile") {
					return writeDockerfile(n.Context.PWD, n.Builds[target])
				}
			}

			return nil
		},
	}
}
