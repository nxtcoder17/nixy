package nixy2

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nxtcoder17/nixy/internal/app"
	"gopkg.in/yaml.v3"
)

func TestColimaHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("COLIMA_HOME", "")

	got, err := colimaHome()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".colima"); got != want {
		t.Fatalf("colimaHome() = %q, want %q", got, want)
	}

	override := filepath.Join(home, "custom-colima")
	t.Setenv("COLIMA_HOME", override)
	got, err = colimaHome()
	if err != nil {
		t.Fatal(err)
	}
	if got != override {
		t.Fatalf("colimaHome() = %q, want %q", got, override)
	}
}

func TestColimaPaths(t *testing.T) {
	t.Setenv("COLIMA_HOME", filepath.Join(t.TempDir(), "colima"))
	c := &colimaExecutor{profileName: "nixy-test"}

	configDir, err := c.colimaConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(os.Getenv("COLIMA_HOME"), "nixy-test"); configDir != want {
		t.Fatalf("colimaConfigDir() = %q, want %q", configDir, want)
	}

	sshPath, err := c.sshConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(os.Getenv("COLIMA_HOME"), "_lima", "colima-nixy-test", "ssh.config"); sshPath != want {
		t.Fatalf("sshConfigPath() = %q, want %q", sshPath, want)
	}
}

func TestNewColimaExecutorGeneratesConfiguredYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "colima.yaml")
	n := &Nixy{
		NixyYAML: &NixyYAML{
			ColimaModeConfig: ColimaModeConfig{
				CPU:    6,
				Memory: 12,
				Disk:   80,
				Ports:  []string{"8080:80", "3000"},
			},
		},
	}
	c := n.NewColimaExecutor(&app.Context{Env: &app.Env{}, PWD: tmpDir}, &FSPaths{GeneratedColimaConfigFilePath: configPath}).(*colimaExecutor)
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("COLIMA_HOME", filepath.Join(tmpDir, "colima"))
	t.Setenv("PATH", filepath.Join(tmpDir, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	colimaScript := "#!/bin/sh\ncase \"$1\" in\nstatus) exit 1 ;;\nstart) mkdir -p \"$(dirname \"$TEST_SSH_CONFIG\")\"; : > \"$TEST_SSH_CONFIG\" ;;\nssh) exit 0 ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(binDir, "colima"), []byte(colimaScript), 0755); err != nil {
		t.Fatal(err)
	}
	sshPath := filepath.Join(os.Getenv("COLIMA_HOME"), "_lima", "colima-"+c.profileName, "ssh.config")
	t.Setenv("TEST_SSH_CONFIG", sshPath)

	if err := c.EnsureColimaVM(&app.Context{Context: context.Background(), PWD: tmpDir}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var got colimaConfig
	if err := yaml.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.CPU != 6 || got.Memory != 12 || got.Disk != 80 {
		t.Fatalf("generated resources = cpu %d, memory %d, disk %d", got.CPU, got.Memory, got.Disk)
	}
	if len(got.PortForwards) != 2 || got.PortForwards[0].HostPort != 8080 || got.PortForwards[0].GuestPort != 80 || got.PortForwards[1].HostPort != 3000 || got.PortForwards[1].GuestPort != 3000 {
		t.Fatalf("generated ports = %+v", got.PortForwards)
	}
}

func TestMergeNixyYAMLsColimaModeConfig(t *testing.T) {
	got := mergeNixyYAMLs(
		&NixyYAML{ColimaModeConfig: ColimaModeConfig{CPU: 2, Ports: []string{"8080:80"}}},
		&NixyYAML{ColimaModeConfig: ColimaModeConfig{Memory: 8, Disk: 50, Ports: []string{"3000"}}},
	)
	if got.ColimaModeConfig.CPU != 2 || got.ColimaModeConfig.Memory != 8 || got.ColimaModeConfig.Disk != 50 {
		t.Fatalf("merged resources = %+v", got.ColimaModeConfig)
	}
	if len(got.ColimaModeConfig.Ports) != 2 {
		t.Fatalf("merged ports = %+v", got.ColimaModeConfig.Ports)
	}
}

func TestEnsureColimaVMSSHConfig(t *testing.T) {
	tests := []struct {
		name       string
		createSSH  bool
		wantErrMsg string
	}{
		{name: "config exists", createSSH: true},
		{name: "config missing", wantErrMsg: "Colima SSH config not found at"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			colimaHome := filepath.Join(tmpDir, "colima")
			binDir := filepath.Join(tmpDir, "bin")
			if err := os.MkdirAll(binDir, 0755); err != nil {
				t.Fatal(err)
			}

			profile := "nixy-test"
			sshPath := filepath.Join(colimaHome, "_lima", "colima-"+profile, "ssh.config")
			colimaScript := `#!/bin/sh
case "$1" in
status) exit 1 ;;
start)
  if [ "$CREATE_SSH" = "1" ]; then
    mkdir -p "$(dirname "$TEST_SSH_CONFIG")"
    : > "$TEST_SSH_CONFIG"
  fi
  exit 0 ;;
ssh) exit 0 ;;
esac
exit 1
`
			colimaPath := filepath.Join(binDir, "colima")
			if err := os.WriteFile(colimaPath, []byte(colimaScript), 0755); err != nil {
				t.Fatal(err)
			}

			t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("COLIMA_HOME", colimaHome)
			t.Setenv("TEST_SSH_CONFIG", sshPath)
			if tt.createSSH {
				t.Setenv("CREATE_SSH", "1")
			} else {
				t.Setenv("CREATE_SSH", "0")
			}

			configDir := filepath.Join(tmpDir, ".nixy")
			if err := os.MkdirAll(configDir, 0755); err != nil {
				t.Fatal(err)
			}
			configPath := filepath.Join(configDir, "colima.yaml")
			c := &colimaExecutor{
				profileName:  profile,
				colimaConfig: configPath,
			}
			ctx := &app.Context{
				Context: context.Background(),
				PWD:     tmpDir,
			}

			err := c.EnsureColimaVM(ctx)
			if tt.wantErrMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrMsg) || !strings.Contains(err.Error(), sshPath) {
					t.Fatalf("EnsureColimaVM() error = %v, want %q containing %q", err, tt.wantErrMsg, sshPath)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(sshPath); err != nil {
				t.Fatalf("expected SSH config at %s: %v", sshPath, err)
			}
		})
	}
}
