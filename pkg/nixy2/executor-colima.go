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
	Arch         string              `yaml:"arch,omitempty"`
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
	Arch         string
	Ports        []colimaConfigPort
	colimaConfig string
}

func GenColimaProfileName(pwd string) string {
	hasher := sha256.New()
	hasher.Write([]byte(pwd))
	profileHash := fmt.Sprintf("%x", hasher.Sum(nil))[:12]
	return fmt.Sprintf("nixy-%s", profileHash)
}

// genColimaProfileNameWithArch expects arch to be a normalized colima arch
// ("x86_64"/"aarch64"), or "" when the VM uses the host arch.
func genColimaProfileNameWithArch(pwd, arch string) string {
	if arch == "" {
		return GenColimaProfileName(pwd)
	}
	hasher := sha256.New()
	hasher.Write([]byte(pwd))
	hasher.Write([]byte("|arch="))
	hasher.Write([]byte(arch))
	profileHash := fmt.Sprintf("%x", hasher.Sum(nil))[:12]
	return fmt.Sprintf("nixy-%s", profileHash)
}

func colimaHome() (string, error) {
	if home := os.Getenv("COLIMA_HOME"); home != "" {
		return home, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".colima"), nil
}

func (c *colimaExecutor) colimaConfigDir() (string, error) {
	home, err := colimaHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, c.profileName), nil
}

func (c *colimaExecutor) sshConfigPath() (string, error) {
	home, err := colimaHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "_lima", "colima-"+c.profileName, "ssh.config"), nil
}

func (n *Nixy) NewColimaExecutor(appCtx *app.Context, fsPaths *FSPaths) Executor {
	// arch stays empty unless the user explicitly asked for one, since empty
	// means "use the host arch" (and keeps the profile name host-agnostic).
	arch := strings.ToLower(strings.TrimSpace(n.ColimaModeConfig.Arch))
	if arch != "" && arch != "host" {
		arch = ParseCPUArch(arch).ColimaARCH()
	} else {
		arch = ""
	}

	profileName := genColimaProfileNameWithArch(appCtx.PWD, arch)

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

	env := fn.MapMerge(n.Env, commonExecutorEnv(appCtx))

	return &colimaExecutor{
		profileName:  profileName,
		Env:          env,
		Mounts:       n.Mounts,
		CPU:          cpu,
		Memory:       memory,
		Disk:         disk,
		Arch:         arch,
		Ports:        ports,
		colimaConfig: fsPaths.GeneratedColimaConfigFilePath,
	}
}

func (c *colimaExecutor) EnsureColimaVM(appCtx *app.Context) error {
	statusCmd := exec.CommandContext(appCtx.Context, "colima", "status", "-p", c.profileName)
	vmRunning := statusCmd.Run() == nil

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
		Arch:         c.Arch,
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

	targetConfigDir, err := c.colimaConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(targetConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create colima config dir: %w", err)
	}

	targetConfigPath := filepath.Join(targetConfigDir, "colima.yaml")
	_ = os.Remove(targetConfigPath)
	if err := os.Symlink(c.colimaConfig, targetConfigPath); err != nil {
		return fmt.Errorf("failed to symlink colima config: %w", err)
	}

	if !vmRunning {
		fastlog.Info("Starting Colima VM profile...", "profile", c.profileName)
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

	sshConfigPath, err := c.sshConfigPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(sshConfigPath); err != nil {
		return fmt.Errorf("Colima SSH config not found at %s: %w", sshConfigPath, err)
	}

	// Bake host version into script as fallback when $NIXY_VERSION is not propagated to the VM
	hostVersion := strings.ReplaceAll(appCtx.AppVersion, "%", "%%")
	// Now check and install Nix inside the VM if not present
	checkNixScript := fmt.Sprintf(`
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

if ! command -v nixy >/dev/null; then
  echo "nixy not found in VM. Installing..."
  mkdir -p ~/.local/bin/
  # Resolve arch from inside the VM: nixy release artifacts are named by go arch
  case "$(uname -m)" in
    x86_64|amd64) nixy_arch="amd64" ;;
    aarch64|arm64) nixy_arch="arm64" ;;
    *) nixy_arch="amd64" ;;
  esac
  # Resolve version-aware download URL: use $NIXY_VERSION (or baked host version) when it looks like a release tag (v*), otherwise fall back to latest
  NIXY_VER="$NIXY_VERSION"
  if [ -z "$NIXY_VER" ]; then
    NIXY_VER="%s"
  fi
  if [ -z "$NIXY_VER" ] || echo "$NIXY_VER" | grep -qE "^(dev|nightly)"; then
    url="https://github.com/nxtcoder17/nixy/releases/latest/download/nixy-linux-$nixy_arch"
  else
    case "$NIXY_VER" in v*) ver="$NIXY_VER" ;; *) ver="v$NIXY_VER" ;; esac
    url="https://github.com/nxtcoder17/nixy/releases/download/$ver/nixy-linux-$nixy_arch"
    # If the versioned asset 404s (e.g. stale pre-release tag), fall back to latest
    if ! curl --fail --silent --head --location "$url" >/dev/null 2>&1; then
      echo "Versioned nixy asset not found at $url, falling back to latest"
      url="https://github.com/nxtcoder17/nixy/releases/latest/download/nixy-linux-$nixy_arch"
    fi
  fi
  echo "Downloading nixy from $url (arch=$nixy_arch, version=$NIXY_VER)..."
  curl --fail --silent --show-error --location --retry 3 "$url" --output ~/.local/bin/nixy
  chmod +x ~/.local/bin/nixy
fi
`, hostVersion)
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

func shouldUseKittySSH() bool {
	if os.Getenv("TERM") != "xterm-kitty" {
		return false
	}
	// kitty's remote control breaks inside tmux/screen — fall back to plain ssh
	if os.Getenv("TMUX") != "" {
		return false
	}
	if os.Getenv("STY") != "" { // GNU screen
		return false
	}
	// Require kitty context vars; without them kitty +kitten ssh fails with
	// "Incorrect request ... expecting KITTY_PID/KITTY_WINDOW_ID"
	if os.Getenv("KITTY_PID") == "" && os.Getenv("KITTY_WINDOW_ID") == "" && os.Getenv("KITTY_LISTEN_ON") == "" {
		return false
	}
	if _, err := exec.LookPath("kitty"); err != nil {
		return false
	}
	return true
}

func (c *colimaExecutor) Exec(appCtx *app.Context, cmd string, args ...string) (*exec.Cmd, error) {
	if err := c.EnsureColimaVM(appCtx); err != nil {
		return nil, err
	}

	sshConfigPath, err := c.sshConfigPath()
	if err != nil {
		return nil, err
	}

	var sshCmd []string
	if shouldUseKittySSH() {
		sshCmd = []string{"kitty", "+kitten", "ssh"}
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

	sshConfigPath, err := c.sshConfigPath()
	if err != nil {
		return err
	}

	if raw {
		fmt.Printf("ssh -F %s lima-colima-%s\n", sshConfigPath, c.profileName)
		return nil
	}

	sshCmd := func() []string {
		if shouldUseKittySSH() {
			return []string{"kitty", "+kitten", "ssh"}
		}

		return []string{"ssh"}
	}()

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
