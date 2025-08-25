package nix

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

func (n *Nix) Shell(ctx context.Context, shell string) error {
	if shell == "" {
		v, ok := os.LookupEnv("SHELL")
		if !ok {
			return fmt.Errorf("must specify a valid shell, in case SHELL env-var is not defined")
		}
		shell = filepath.Base(v)
	}

	envOverride := []string{
		"IN_NIXY_SHELL=true",
	}

	t := template.New("project-flake")
	if _, err := t.Parse(templateProjectFlake); err != nil {
		return err
	}

	// dir := ".nixy"
	dir := filepath.Join(n.ProfileSetupDir(), "project-dir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(dir, "flake.nix"))
	if err != nil {
		return err
	}

	nixPkgs := make(map[string][]string)
	var nightlyPkgs []string

	for _, pkg := range n.Packages {
		if !strings.HasPrefix(pkg, "nixpkgs/") {
			pkg = fmt.Sprintf("nixpkgs/%s#%s", n.NixPkgs, pkg)
		}
		if strings.HasPrefix(pkg, "nixpkgs/") {
			sp := strings.Split(pkg, "#")
			if len(sp) != 2 {
				continue
			}

			key := sp[0][len("nixpkgs/"):]

			nixPkgs[key] = append(nixPkgs[key], sp[1])
			continue
		}
		nightlyPkgs = append(nightlyPkgs, pkg)
	}

	workspaceDir, err := os.Getwd()
	if err != nil {
		return err
	}

	profileSetupDir := n.ProfileSetupDir()
	if n.Executor == BubbleWrapExecutor {
		profileSetupDir = "/nixy-profile"
	}

	if err := t.ExecuteTemplate(f, "project-flake", map[string]any{
		"nixPkgs":       nixPkgs,
		"nightlyPkgs":   nightlyPkgs,
		"projectDir":    workspaceDir,
		"profileDir":    profileSetupDir,
		"nixpkgsCommit": n.NixPkgs,
	}); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(dir, ".envrc"), []byte(`use flake .`), 0o755); err != nil {
		return err
	}

	args := []string{
		"shell", fmt.Sprintf("nixpkgs/%s#bash", n.NixPkgs), "--command", "bash", "-c",
		`
pushd $NIXY_PROFILE_DIR/project-dir
cat flake.nix
nix develop --override-input profile-flake $NIXY_PROFILE_DIR --command fish
popd
`,
	}
	// args = append(args, "--command", shell)

	cmd, err := n.PrepareNixCommand(ctx, "nix", args)
	if err != nil {
		return err
	}

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
