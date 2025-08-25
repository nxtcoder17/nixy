package nix

import (
	"bufio"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// var (
// 	BubbleWrapConfigHome
// )

func profilePath(name string) string {
	return filepath.Join(XDGDataDir(), "profiles", name)
}

func ProfileList(ctx context.Context) ([]string, error) {
	de, err := os.ReadDir(filepath.Join(XDGDataDir(), "profiles"))
	if err != nil {
		return nil, err
	}

	profiles := make([]string, 0, len(de))

	for i := range de {
		if de[i].Type().IsDir() {
			profiles = append(profiles, de[i].Name())
		}
	}

	return profiles, nil
}

func downloadStaticNixBinary(ctx context.Context, binPath string) error {
	_, err := os.Stat(binPath)
	if err == nil {
		fmt.Println("PATH already exists")
		return nil
	}

	if !askUser(fmt.Sprintf("Downloading Static Nix Binary to %s ? ", binPath)) {
		return fmt.Errorf("User did not allow downloading static nix binary")
	}

	url := "https://hydra.nixos.org/job/nix/master/buildStatic.nix-cli.x86_64-linux/latest/download/1"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	output, err := os.OpenFile(binPath,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, // flags
		0o755,                              // permissions
	)
	if err != nil {
		return err
	}

	totalBytes := 0

	b := make([]byte, 0xffff)
	for {
		n, err := resp.Body.Read(b)
		totalBytes += n
		fmt.Printf("\033[2K\rDownloading Static Nix Executable ... %.2fMBs", float64(totalBytes)/1024/1024)
		if _, err := output.Write(b[:n]); err != nil {
			return fmt.Errorf("failed to write to output path: %w", err)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Printf("\n")
				break
			}
			return err
		}
	}

	output.Close()
	resp.Body.Close()

	return nil
}

func askUser(message string) bool {
	fmt.Printf("%s (Y/N): ", message)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input) // remove newline + spaces

	switch strings.ToLower(input) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

//go:embed templates/profile-flake.nix.tpl
var templateProfileFlake string

//go:embed templates/project-flake.nix.tpl
var templateProjectFlake string

func ProfileCreate(ctx context.Context, name string) error {
	n := Nix{Executor: BubbleWrapExecutor, BubbleWrap: BubbleWrap{ProfileName: name}}
	n.BubbleWrap.createFSDirs()

	if err := downloadStaticNixBinary(ctx, filepath.Join(n.BubbleWrap.ProfileBinPath(), "nix")); err != nil {
		return err
	}

	args := []string{
		"-c",
		fmt.Sprintf(`
pushd %s
cat > flake.nix <<EOF
%s
EOF
popd
`, n.ProfileSetupDir(), templateProfileFlake),
	}

	// Downloading Static Nix Binary
	cmd := exec.CommandContext(ctx, "bash", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func ProfileEdit(ctx context.Context, name string) error {
	n := Nix{Executor: BubbleWrapExecutor, BubbleWrap: BubbleWrap{ProfileName: name}}
	cmd := exec.CommandContext(ctx, os.Getenv("EDITOR"), "flake.nix")
	cmd.Dir = filepath.Join(n.ProfileSetupDir())
	cmd.Stdout = os.Stdout
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
