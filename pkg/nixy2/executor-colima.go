package nixy2

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nxtcoder17/fastlog"
	"github.com/nxtcoder17/nixy/internal/app"
	"gopkg.in/yaml.v3"
)

type colimaConfig struct {
	CPU          int                 `yaml:"cpu"`
	Memory       int                 `yaml:"memory"`
	Runtime      string              `yaml:"runtime"`
	Mounts       []colimaConfigMount `yaml:"mounts,omitempty"`
	PortForwards []colimaConfigPort  `yaml:"portForwards,omitempty"`
}

type colimaConfigMount struct {
	Location   string `yaml:"location"`
	MountPoint string `yaml:"mountPoint,omitempty"`
	Writable   bool   `yaml:"writable"`
}

type colimaConfigPort struct {
	GuestPort int    `yaml:"guestPort"`
	HostPort  int    `yaml:"hostPort"`
	Proto     string `yaml:"proto,omitempty"`
}

type colimaExecutor struct {
	profileName  string
	Env          map[string]string
	Mounts       []NixyMount
	CPU          int
	Memory       int
	Ports        []colimaConfigPort
	colimaConfig string
}

func (n *Nixy) NewColimaExecutor(appCtx *app.Context, fsPaths *FSPaths) Executor {
	hasher := sha256.New()
	hasher.Write([]byte(appCtx.PWD))
	profileHash := fmt.Sprintf("%x", hasher.Sum(nil))[:12]
	profileName := fmt.Sprintf("nixy-%s", profileHash)

	cpu := n.ColimaModeConfig.CPU
	if cpu <= 0 {
		cpu = 2
	}
	memory := n.ColimaModeConfig.Memory
	if memory <= 0 {
		memory = 2
	}

	var ports []colimaConfigPort
	for _, v := range n.ColimaModeConfig.Ports {
		sp := strings.Split(v, ":")
		if len(sp) > 2 {
			continue
		}
		var hp, cp int
		if len(sp) == 1 {
			portVal, err := strconv.Atoi(sp[0])
			if err == nil {
				hp = portVal
				cp = portVal
			}
		} else if len(sp) == 2 {
			hpVal, err1 := strconv.Atoi(sp[0])
			cpVal, err2 := strconv.Atoi(sp[1])
			if err1 == nil && err2 == nil {
				hp = hpVal
				cp = cpVal
			}
		}
		if hp > 0 && cp > 0 {
			ports = append(ports, colimaConfigPort{HostPort: hp, GuestPort: cp, Proto: "tcp"})
		}
	}

	return &colimaExecutor{
		profileName:  profileName,
		Env:          n.Env,
		Mounts:       n.Mounts,
		CPU:          cpu,
		Memory:       memory,
		Ports:        ports,
		colimaConfig: fsPaths.GeneratedColimaConfigFilePath,
	}
}

func (c *colimaExecutor) ensureColimaVM(appCtx *app.Context) error {
	statusCmd := exec.CommandContext(appCtx.Context, "colima", "status", "-p", c.profileName)
	if err := statusCmd.Run(); err != nil {
		// VM is not running, start it
		fastlog.Info("Starting Colima VM profile...", "profile", c.profileName)

		// Create mounts list
		var mounts []colimaConfigMount
		mounts = append(mounts, colimaConfigMount{
			Location: appCtx.PWD,
			Writable: true,
		})
		for _, m := range c.Mounts {
			mounts = append(mounts, colimaConfigMount{
				Location:   m.Source,
				MountPoint: m.Destination,
				Writable:   !m.ReadOnly,
			})
		}

		cfg := colimaConfig{
			CPU:          c.CPU,
			Memory:       c.Memory,
			Runtime:      "docker",
			Mounts:       mounts,
			PortForwards: c.Ports,
		}

		b, err := yaml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("failed to marshal colima config: %w", err)
		}

		if err := os.WriteFile(c.colimaConfig, b, 0644); err != nil {
			return fmt.Errorf("failed to write colima config to %s: %w", c.colimaConfig, err)
		}

		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		targetConfigDir := filepath.Join(homeDir, ".config", "colima", c.profileName)
		if err := os.MkdirAll(targetConfigDir, 0755); err != nil {
			return fmt.Errorf("failed to create colima config dir: %w", err)
		}

		targetConfigPath := filepath.Join(targetConfigDir, "colima.yaml")
		_ = os.Remove(targetConfigPath) // Remove if it exists or is a broken symlink
		if err := os.Symlink(c.colimaConfig, targetConfigPath); err != nil {
			return fmt.Errorf("failed to symlink colima config: %w", err)
		}

		cmd := exec.CommandContext(appCtx.Context, "colima", "start", c.profileName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		fastlog.Debug("Starting Colima VM", "command", cmd.String())
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

	resCmd.Env = append(resCmd.Env, "TERM=xterm")

	return resCmd, nil
}
