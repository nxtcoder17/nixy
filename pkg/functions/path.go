package functions

import (
	"os"
)

func pathExists(path string) (os.FileInfo, bool) {
	stat, err := os.Lstat(path)
	if err == nil {
		return stat, true
	}

	return nil, false
}

func PathExists(path string) bool {
	_, ok := pathExists(path)
	return ok
}

func PathIsDirectory(path string) bool {
	stat, ok := pathExists(path)
	if !ok {
		return false
	}

	return stat.IsDir()
}

func PathIsFile(path string) bool {
	stat, ok := pathExists(path)
	if !ok {
		return false
	}

	return !stat.IsDir()
}
