package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/nxtcoder17/nixy/internal/app"
	"github.com/urfave/cli/v3"
)

func actionEnv() *cli.Command {
	return &cli.Command{
		Name:    "env",
		Suggest: true,
		Usage:   "list the environment variables nixy reads",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "json",
				Usage: "print the list as JSON",
			},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			return writeEnvVars(os.Stdout, app.EnvVars(), c.Bool("json"))
		},
	}
}

func writeEnvVars(w io.Writer, vars []app.EnvVar, asJSON bool) error {
	if asJSON {
		b, err := json.MarshalIndent(vars, "", "  ")
		if err != nil {
			return err
		}

		_, err = fmt.Fprintln(w, string(b))
		return err
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "NAME\tDEFAULT\tCURRENT\tUSAGE"); err != nil {
		return err
	}

	for _, v := range vars {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", v.Name, orDash(v.Default), orDash(v.Value), v.Usage); err != nil {
			return err
		}
	}

	return tw.Flush()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}

	return s
}
