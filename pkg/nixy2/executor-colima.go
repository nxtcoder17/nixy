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
	fn "github.com/nxtcoder17/nixy/pkg/functions"
	"gopkg.in/yaml.v3"
)

type colimaConfig struct {
	CPU          int                 `yaml:"cpu"`
	Disk         int                 `yaml:"disk"`
	Memory       int                 `yaml:"memory"`
	Runtime      string              `yaml:"runtime"`
	Mounts       []colimaConfigMount `yaml:"mounts,omitempty"`
	PortForwards []colimaConfigPort  `yaml:"portForwards,omitempty"`
	Env          map[string]string   `yaml:"env,omitempty"`
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
	Disk         int
	Ports        []colimaConfigPort
	colimaConfig string
}

func GenColimaProfileName(pwd string) string {
	hasher := sha256.New()
	hasher.Write([]byte(pwd))
	profileHash := fmt.Sprintf("%x", hasher.Sum(nil))[:12]
	return fmt.Sprintf("nixy-%s", profileHash)
}

func (n *Nixy) NewColimaExecutor(appCtx *app.Context, fsPaths *FSPaths) Executor {
	profileName := GenColimaProfileName(appCtx.PWD)

	cpu := n.ColimaModeConfig.CPU
	if cpu <= 0 {
		cpu = 2
	}
	memory := n.ColimaModeConfig.Memory
	if memory <= 0 {
		memory = 2
	}
	disk := n.ColimaModeConfig.Disk
	if disk <= 0 {
		disk = 20
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
		Env:          fn.MapMerge(n.Env, commonExecutorEnv(appCtx)),
		Mounts:       n.Mounts,
		CPU:          cpu,
		Memory:       memory,
		Disk:         disk,
		Ports:        ports,
		colimaConfig: fsPaths.GeneratedColimaConfigFilePath,
	}
}

func (c *colimaExecutor) EnsureColimaVM(appCtx *app.Context) error {
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
			Disk:         c.Disk,
			Memory:       c.Memory,
			Runtime:      "docker",
			Mounts:       mounts,
			PortForwards: c.Ports,
			Env:          c.Env,
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
		cmd.Env = append(cmd.Env, os.Environ()...)
		cmd.Env = append(cmd.Env, fn.ToEnviron(c.Env)...)
		fastlog.Debug("Starting Colima VM", "command", cmd.String(), "env", cmd.Env)
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

func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func (c *colimaExecutor) Exec(appCtx *app.Context, cmd string, args ...string) (*exec.Cmd, error) {
	if err := c.EnsureColimaVM(appCtx); err != nil {
		return nil, err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	sshConfigPath := filepath.Join(homeDir, ".config", "colima", "_lima", "colima-"+c.profileName, "ssh.config")

	var sshCmd []string
	if os.Getenv("TERM") == "xterm-kitty" {
		if _, err := exec.LookPath("kitty"); err == nil {
			sshCmd = []string{"kitty", "+kitten", "ssh"}
		}
	}
	if len(sshCmd) == 0 {
		sshCmd = []string{"ssh"}
	}

	escapedArgs := make([]string, len(args))
	for i, arg := range args {
		escapedArgs[i] = shellEscape(arg)
	}

	remoteScript := fmt.Sprintf(`. ~/.nix-profile/etc/profile.d/nix.sh
cd %s
exec %s %s`,
		shellEscape(appCtx.PWD),
		shellEscape(cmd),
		strings.Join(escapedArgs, " "),
	)

	sshArgs := append(sshCmd,
		"-t",
		"-F", sshConfigPath,
		"lima-colima-"+c.profileName,
		"--",
		remoteScript,
	)

	fastlog.Debug("Colima Native SSH Command Ready", "args", sshArgs)
	resCmd := exec.CommandContext(appCtx.Context, sshArgs[0], sshArgs[1:]...)

	// Also pass any env vars specified in nixy.yml
	for k, v := range c.Env {
		resCmd.Env = append(resCmd.Env, k+"="+v)
	}

	resCmd.Env = append(resCmd.Env, "TERM="+os.Getenv("TERM"))

	return resCmd, nil
}

func (c *colimaExecutor) Stop(appCtx *app.Context) error {
	fastlog.Info("Stopping Colima VM profile...", "profile", c.profileName)

	cmd := exec.CommandContext(appCtx.Context, "colima", "stop", c.profileName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c *colimaExecutor) SSH(appCtx *app.Context, raw bool) error {
	if !raw {
		if err := c.EnsureColimaVM(appCtx); err != nil {
			return err
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	sshConfigPath := filepath.Join(homeDir, ".config", "colima", "_lima", "colima-"+c.profileName, "ssh.config")

	if raw {
		fmt.Printf("ssh -F %s lima-colima-%s\n", sshConfigPath, c.profileName)
		return nil
	}

	var sshCmd []string
	if os.Getenv("TERM") == "xterm-kitty" {
		if _, err := exec.LookPath("kitty"); err == nil {
			sshCmd = []string{"kitty", "+kitten", "ssh"}
		}
	}
	if len(sshCmd) == 0 {
		sshCmd = []string{"ssh"}
	}

	sshArgs := append(sshCmd,
		"-t",
		"-F", sshConfigPath,
		"lima-colima-"+c.profileName,
	)

	cmd := exec.CommandContext(appCtx.Context, sshArgs[0], sshArgs[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
