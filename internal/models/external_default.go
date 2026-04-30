//go:build !android

package models

import (
	"os"

	shared "github.com/asolopovas/wt/internal"
)

func externalRoot() string {
	if v := os.Getenv("WT_MODELS_DIR"); v != "" {
		return v
	}
	return shared.ModelsDir()
}
