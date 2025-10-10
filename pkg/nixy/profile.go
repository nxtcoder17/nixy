package nixy

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProfileList returns all available profiles
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

// ProfileCreate creates a new profile with the given name
func ProfileCreate(ctx context.Context, name string) error {
	nixyCtx, err := NewContext(ctx, "")
	if err != nil {
		return err
	}

	_, err = NewProfile(nixyCtx, name)
	return err
}

// ProfileEdit opens the profile's flake.nix in the user's editor
func ProfileEdit(ctx context.Context, name string) error {
	nixyCtx, err := NewContext(ctx, "")
	if err != nil {
		return err
	}

	if name == "" {
		name = nixyCtx.NixyProfile
	}

	profile, err := NewProfile(nixyCtx, name)
	if err != nil {
		return err
	}

	// cmd := exec.CommandContext(ctx, os.Getenv("EDITOR"), "flake.nix")
	cmd := exec.CommandContext(ctx, os.Getenv("EDITOR"), "nixy.yml")
	cmd.Dir = profile.ProfilePath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

// fetchCurrentNixpkgsHash fetches the latest nixpkgs commit hash
func fetchCurrentNixpkgsHash(ctx context.Context) (string, error) {
	if !askUser("Fetching Current NixPkgs Version ?") {
		return "", fmt.Errorf("User Aborted fetching current nixpkgs version")
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/nixos/nixpkgs/commits/nixos-unstable", nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return "", err
	}

	var result struct {
		SHA string `json:"sha"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	resp.Body.Close()

	return result.SHA, nil
}

// downloadStaticNixBinary downloads the static nix binary for bubblewrap profiles
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

// askUser prompts the user for a yes/no response
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
