//go:build android

package models

import (
	"os"
	"path/filepath"
)

const androidPackage = "com.asolopovas.wtranscribe"

func externalRoot() string {
	if v := os.Getenv("WT_MODELS_DIR"); v != "" {
		return v
	}
	if v := os.Getenv("EXTERNAL_STORAGE"); v != "" {
		return filepath.Join(v, "Android", "data", androidPackage, "files", "models")
	}
	return filepath.Join("/sdcard", "Android", "data", androidPackage, "files", "models")
}
