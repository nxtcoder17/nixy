package nix

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type MountedDir struct {
	HostPath    string
	MountedPath string
}

type BubbleWrapProfile struct {
	HostPath         string
	MetadataHostPath string

	ProfileFlakeDir MountedDir

	UserHome      MountedDir
	NixDir        MountedDir
	NixBinDir     string
	WorkspacesDir MountedDir
}

func UseBubbleWrap(profileName string) BubbleWrapProfile {
	if profileName == "" {
		if v, ok := os.LookupEnv("NIXY_PROFILE"); ok {
			profileName = v
		} else {
			profileName = "default"
		}
	}

	profilePath := filepath.Join(XDGDataDir(), "profiles", profileName)

	nixDirHostPath := filepath.Join(profilePath, "nix")

	return BubbleWrapProfile{
		HostPath:         profilePath,
		MetadataHostPath: filepath.Join(profilePath, "metadata.json"),

		ProfileFlakeDir: MountedDir{
			HostPath:    filepath.Join(profilePath, "profile-flake"),
			MountedPath: "/profile",
		},

		UserHome: MountedDir{
			HostPath:    filepath.Join(profilePath, "home"),
			MountedPath: "/home/nixy",
		},
		NixDir: MountedDir{
			HostPath:    nixDirHostPath,
			MountedPath: "/nix",
		},
		NixBinDir: filepath.Join(nixDirHostPath, "bin"),
		WorkspacesDir: MountedDir{
			HostPath:    filepath.Join(profilePath, "workspaces"),
			MountedPath: "/workspaces",
		},
	}
}

type ProfileMetadata struct {
	NixPkgsCommit string `json:"nixpkgs"`
}

func (b *BubbleWrapProfile) writeProfileMetadata(p ProfileMetadata) error {
	b2, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(b.MetadataHostPath, b2, 0o755)
}

func (b *BubbleWrapProfile) ProfileMetadata() (*ProfileMetadata, error) {
	meta, err := os.ReadFile(b.MetadataHostPath)
	if err != nil {
		return nil, err
	}

	var metadata ProfileMetadata

	if err := json.Unmarshal(meta, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

func (b *BubbleWrapProfile) createFSDirs() error {
	dirs := []string{b.UserHome.HostPath, b.NixDir.HostPath, b.NixBinDir, b.WorkspacesDir.HostPath, b.ProfileFlakeDir.HostPath}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return nil
}
