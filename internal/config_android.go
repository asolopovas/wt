//go:build android

package shared

import (
	"os"
	"path/filepath"
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
