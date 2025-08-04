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
	"syscall"
	"time"

	"github.com/nxtcoder17/fastlog"
	"github.com/nxtcoder17/nixy/pkg/nix"
	"github.com/urfave/cli/v3"
)

var Version string

func main() {
	if Version == "" {
		Version = fmt.Sprintf("nightly | %s", time.Now().Format(time.RFC3339))
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

		Commands: []*cli.Command{
			{
				Name:    "init",
				Suggest: true,
				Action: func(ctx context.Context, c *cli.Command) error {
					if _, err := os.Stat("nixy.yml"); err != nil {
						if errors.Is(err, fs.ErrNotExist) {
							if err := os.WriteFile("nixy.yml", []byte(`packages: []`), 0o644); err != nil {
								return err
							}
							return nil
						}
						return err
					}

					return nil
				},
			},
			{
				Name:    "install",
				Usage:   "<pkgname>",
				Suggest: true,
				Action: func(ctx context.Context, c *cli.Command) error {
					n, err := loadFromNixyfile(c)
					if err != nil {
						return err
					}

					defer n.SyncToDisk()
					if err := n.Install(ctx, c.Args().Slice()...); err != nil {
						return err
					}
					return nil
				},
			},
			{
				Name:    "shell",
				Suggest: true,
				Action: func(ctx context.Context, c *cli.Command) error {
					n, err := loadFromNixyfile(c)
					if err != nil {
						return err
					}
					defer n.SyncToDisk()

					if err := n.Shell(ctx, c.Args().First()); err != nil {
						return err
					}

					return nil
				},
			},
		},

		Suggest: true,
	}

	ctx, cf := signal.NotifyContext(context.TODO(), syscall.SIGINT, syscall.SIGTERM)
	defer cf()

	go func() {
		<-ctx.Done()
		cf()
	}()

	if err := cmd.Run(ctx, os.Args); err != nil {
		slog.Error("while running cmd, got", "err", err)
	}
}

func loadFromNixyfile(c *cli.Command) (*nix.Nix, error) {
	logger := fastlog.New(fastlog.Options{
		Writer:        os.Stderr,
		Format:        fastlog.ConsoleFormat,
		ShowDebugLogs: c.IsSet("debug"),
		ShowCaller:    true,
		EnableColors:  true,
	})

	slog.SetDefault(logger.Slog())

	switch {
	case c.IsSet("file"):
		return nix.LoadFromFile(c.String("file"))
	default:
		dir, err := os.Getwd()
		if err != nil {
			return nil, err
		}

		oldDir := ""

		nixyConfigFiles := []string{
			"nixy.yml",
		}

		for oldDir != dir {
			for _, fn := range nixyConfigFiles {
				if _, err := os.Stat(filepath.Join(dir, fn)); err != nil {
					if !os.IsNotExist(err) {
						return nil, err
					}
					continue
				}

				return nix.LoadFromFile(filepath.Join(dir, fn))
			}

			oldDir = dir
			dir = filepath.Dir(dir)
		}

		return nil, fmt.Errorf("failed to locate your nearest Nixyfile")
	}
}
