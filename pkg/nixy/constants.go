package nixy

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

var profileBasePath = filepath.Join(XDGDataDir(), "profiles")

func profilePath(profile string) string {
	return filepath.Join(profileBasePath, profile)
}

func flakeDirPath(profile string) string {
	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	h := md5.New()
	h.Write([]byte(pwd))

	return filepath.Join(profilePath(profile), "workspaces", fmt.Sprintf("%x-%s", h.Sum(nil), filepath.Base(pwd)))
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
