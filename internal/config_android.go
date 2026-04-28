//go:build android

package shared

import (
	"os"
	"path/filepath"
	"runtime"
)

func appDir() string {
	filesDir := os.Getenv("FILESDIR")
	if filesDir != "" {
		return filepath.Join(filesDir, "wt")
	}

	return "/data/data/com.asolopovas.wtranscribe/files/wt"
}

func defaultModel() string {
	return "tiny"
}

func defaultThreads() int {
	n := runtime.NumCPU() - 2
	if n < 1 {
		n = 1
	}
	return n
}
