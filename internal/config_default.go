//go:build !android

package shared

import (
	"os"
	"path/filepath"
	"runtime"
)

func appDir() string {
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "wt")
		}
	}

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "wt")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	if runtime.GOOS == "windows" {
		return filepath.Join(home, "AppData", "Roaming", "wt")
	}
	return filepath.Join(home, ".config", "wt")
}

func defaultModel() string {
	return "turbo"
}

func defaultThreads() int {
	return runtime.NumCPU()
}
