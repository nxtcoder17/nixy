package nixy2

import (
	"bufio"
	"context"
	"fmt"
	"github.com/nxtcoder17/go.errors"
	"os"
	"strings"

	"encoding/json"
	"net/http"
)

// askUser prompts the user for a yes/no response
func askUser(message string) bool {
	fmt.Printf("%s (Press Enter to continue): ", message)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input) // remove newline + spaces

	switch strings.ToLower(input) {
	case "":
		return true
	default:
		return false
	}
}

func fetchCurrentNixpkgsHash(ctx context.Context) (string, error) {
	if !askUser("Fetching Current NixPkgs Version ?") {
		return "", errors.New("User aborted fetching current nixpkgs version")
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

func CreateNixyYAML(ctx context.Context) error {
	if !exists("nixy.yml") {
		commit, err := fetchCurrentNixpkgsHash(ctx)
		if err != nil {
			return err
		}

		content := fmt.Sprintf(`
nixpkgs:
  default: "%s"

packages: []
`, commit)

		if err := os.WriteFile("nixy.yml", []byte(strings.TrimSpace(content)), 0644); err != nil {
			return errors.New("failed to write nixy.yml").Wrap(err)
		}
	}

	return nil
}
