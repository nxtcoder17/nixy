package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/nxtcoder17/fastlog"
	"github.com/nxtcoder17/nixy/pkg/nixy"
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

	os.Setenv("NIXY_VERSION", Version)

	var commands []*cli.Command

	if _, ok := os.LookupEnv("NIXY_SHELL"); !ok {
		commands = []*cli.Command{
			{
				Name:    "init",
				Suggest: true,
				Action: func(ctx context.Context, c *cli.Command) error {
					if _, err := os.Stat("nixy.yml"); err != nil {
						if errors.Is(err, fs.ErrNotExist) {
							return nixy.InitNixyFile(ctx, "nixy.yml")
						}
						return err
					}

					return nil
				},
			},
			{
				Name:    "profile",
				Usage:   "<pkgname>",
				Suggest: true,
				Commands: []*cli.Command{
					{
						Name:    "list",
						Aliases: []string{"ls"},
						Action: func(ctx context.Context, c *cli.Command) error {
							profiles, err := nixy.ProfileList(ctx)
							if err != nil {
								return err
							}

							for _, profile := range profiles {
								fmt.Printf("🪪 %s (%s)\n", filepath.Base(profile), profile)
							}
							return nil
						},
					},
					{
						Name: "add",
						Arguments: []cli.Argument{
							&cli.StringArg{
								Name:  "profile-name",
								Value: os.Getenv("NIXY_PROFILE"),
								Config: cli.StringConfig{
									TrimSpace: true,
								},
								UsageText: "Name of Profile",
							},
						},
						Aliases: []string{"new", "create"},
						Action: func(ctx context.Context, c *cli.Command) error {
							profileName := c.StringArg("profile-name")
							if profileName == "" {
								v, ok := os.LookupEnv("NIXY_PROFILE")
								if !ok {
									fmt.Println("Must Specify one argument")
									return nil
								}
								profileName = v
							}

							if err := nixy.ProfileCreate(ctx, c.StringArg("profile-name")); err != nil {
								return err
							}
							return nil
						},
					},
					{
						Name: "edit",
						Arguments: []cli.Argument{
							&cli.StringArg{
								Name: "profile-name",
								Config: cli.StringConfig{
									TrimSpace: true,
								},
								Value:     "",
								UsageText: "Name of Profile",
							},
						},
						Action: func(ctx context.Context, c *cli.Command) error {
							if err := nixy.ProfileEdit(ctx, c.Args().First()); err != nil {
								return err
							}

							return nil
						},
					},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					v, ok := os.LookupEnv("NIXY_PROFILE")
					if !ok {
						v = "default"
					}
					fmt.Println(v)
					return nil
				},
			},
			{
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
			},
			{
				Name:    "shell",
				Suggest: true,
				Action: func(ctx context.Context, c *cli.Command) error {
					n, err := loadFromNixyfile(ctx, c)
					if err != nil {
						return err
					}

					if err := n.Shell(n.Context, strings.Join(c.Args().Slice(), " ")); err != nil {
						return err
					}

					return nil
				},
			},
			{
				Name:    "build",
				Suggest: true,
				Action: func(ctx context.Context, c *cli.Command) error {
					n, err := loadFromNixyfile(ctx, c)
					if err != nil {
						return err
					}

					if err := n.Build(n.Context, c.Args().First()); err != nil {
						return err
					}

					return nil
				},
			},
		}
	} else {
		commands = []*cli.Command{
			{
				Name:    "build",
				Suggest: true,
				Action: func(ctx context.Context, c *cli.Command) error {
					n, err := nixy.LoadInNixyShell(ctx)
					if err != nil {
						return err
					}

					if err := n.Build(ctx, c.Args().First()); err != nil {
						return err
					}

					return nil
				},
			},
		}
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
			logger := fastlog.New(fastlog.Console(), fastlog.ShowDebugLogs(c.Bool("debug")))
			slog.SetDefault(logger.Slog())
			return ctx, nil
		},

		Commands: commands,

		Suggest: true,
	}

	ctx, cf := signal.NotifyContext(context.TODO(), syscall.SIGINT, syscall.SIGTERM)
	defer cf()

	go func() {
		<-ctx.Done()
		cf()
	}()

	if err := cmd.Run(ctx, os.Args); err != nil {
		slog.Error(err.Error())
	}
}

func loadFromNixyfile(ctx context.Context, c *cli.Command) (*nixy.Nixy, error) {
	if c.IsSet("file") {
		return nixy.LoadFromFile(ctx, c.String("file"))
	}

	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	oldDir := ""

	nixyConfigFiles := []string{"nixy.yml"}

	for oldDir != dir {
		for _, fn := range nixyConfigFiles {
			if _, err := os.Stat(filepath.Join(dir, fn)); err != nil {
				if !os.IsNotExist(err) {
					return nil, err
				}
				continue
			}

			return nixy.LoadFromFile(ctx, filepath.Join(dir, fn))
		}

		oldDir = dir
		dir = filepath.Dir(dir)
	}

	return nil, fmt.Errorf("failed to locate your nearest Nixyfile")
}
