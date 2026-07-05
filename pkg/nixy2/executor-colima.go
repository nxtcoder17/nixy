package nixy2

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/nxtcoder17/fastlog"
	"github.com/nxtcoder17/nixy/internal/app"
)

type colimaExecutor struct {
	profileName string
	Env         map[string]string
}

func (n *Nixy) NewColimaExecutor(appCtx *app.Context, fsPaths *FSPaths) Executor {
	hasher := sha256.New()
	hasher.Write([]byte(appCtx.PWD))
	profileHash := fmt.Sprintf("%x", hasher.Sum(nil))[:12]
	profileName := fmt.Sprintf("nixy-%s", profileHash)

	return &colimaExecutor{
		profileName: profileName,
		Env:         n.Env,
	}
}

func (c *colimaExecutor) ensureColimaVM(appCtx *app.Context) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	socketPath := filepath.Join(homeDir, ".colima", c.profileName, "docker.sock")

	if _, err := os.Stat(socketPath); err != nil {
		// VM is not running, start it
		fastlog.Info("Starting Colima VM profile...", "profile", c.profileName)

		startArgs := []string{
			"start",
			c.profileName,
			"--cpu", "2",
			"--memory", "2",
			"--runtime", "docker",
			"--mount", fmt.Sprintf("%s:%s:w", appCtx.PWD, appCtx.PWD),
		}

		cmd := exec.CommandContext(appCtx.Context, "colima", startArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to start Colima VM profile %s: %w", c.profileName, err)
		}
	}

	// Now check and install Nix inside the VM if not present
	checkNixScript := `
if ! command -v xz >/dev/null; then
  echo "xz-utils not found in VM. Installing..."
  sudo apt-get update && sudo apt-get install -y xz-utils
fi
if ! [ -f ~/.nix-profile/etc/profile.d/nix.sh ]; then
  echo "Nix not found or incomplete in VM. Installing Nix..."
  sudo rm -rf /nix
  curl -L https://nixos.org/nix/install | sh -s -- --no-daemon --no-modify-profile
fi
mkdir -p ~/.config/nix
if ! grep -q "experimental-features" ~/.config/nix/nix.conf 2>/dev/null; then
  echo "experimental-features = nix-command flakes" >> ~/.config/nix/nix.conf
fi
`
	cmd := exec.CommandContext(appCtx.Context, "colima", "ssh", "-p", c.profileName, "--", "sh", "-c", checkNixScript)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to initialize Nix inside Colima VM: %w", err)
	}

	return nil
}

func (c *colimaExecutor) Exec(appCtx *app.Context, cmd string, args ...string) (*exec.Cmd, error) {
	if err := c.ensureColimaVM(appCtx); err != nil {
		return nil, err
	}

	// We want to run the command inside the VM.
	// Since Nix is installed in the VM, we must source the Nix environment script
	// before executing the command.
	// The command we run via SSH:
	// . ~/.nix-profile/etc/profile.d/nix.sh && cd <PWD> && exec <cmd> <args...>
	// To do this safely with arguments, we pass them as arguments to the shell:
	// sh -c '. ~/.nix-profile/etc/profile.d/nix.sh && cd "$1" && shift && exec "$@"' -- <PWD> <cmd> <args...>

	sshArgs := []string{
		"ssh",
		"-p", c.profileName,
		"--",
		"sh", "-c",
		`. ~/.nix-profile/etc/profile.d/nix.sh
cd "$1"
shift
exec "$@"`,
		"--",
		appCtx.PWD,
		cmd,
	}
	sshArgs = append(sshArgs, args...)

	fastlog.Debug("Colima Native SSH Command Ready", "args", sshArgs)
	resCmd := exec.CommandContext(appCtx.Context, "colima", sshArgs...)

	// Also pass any env vars specified in nixy.yml
	for k, v := range c.Env {
		resCmd.Env = append(resCmd.Env, k+"="+v)
	}

	return resCmd, nil
}
