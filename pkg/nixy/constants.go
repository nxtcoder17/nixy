package nixy

import (
	"path/filepath"
	"runtime"
)

var profileBasePath = filepath.Join(XDGDataDir(), "profiles")

func profilePath(profile string) string {
	return filepath.Join(profileBasePath, profile)
}

func workspaceNixyDir(workspaceDir string) string {
	return filepath.Join(workspaceDir, ".nixy")
}

var osArchEnv = map[string]string{
	// Nixy Env Vars
	"NIXY_OS":   runtime.GOOS,
	"NIXY_ARCH": runtime.GOARCH,
	"NIXY_ARCH_FULL": func() string {
		switch runtime.GOARCH {
		case "amd64":
			return "x86_64"
		case "386":
			return "i686"
		case "arm64":
			return "aarch64"
		default:
			return runtime.GOARCH
		}
	}(),
}
