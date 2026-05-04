package transcriber

import (
	"fmt"
	"os"
	"path/filepath"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/models"
)

func ResolveModelPathLocal(modelID, override string) (string, error) {
	if override != "" {
		if _, err := os.Stat(override); err == nil {
			return override, nil
		}
	}
	if modelID == "" {
		return "", fmt.Errorf("model id is empty")
	}
	dir := filepath.Join(shared.ModelsDir(), modelID)
	if _, err := os.Stat(dir); err == nil {
		return dir, nil
	}
	if _, err := os.Stat(dir + ".onnx"); err == nil {
		return dir + ".onnx", nil
	}
	return "", fmt.Errorf("model %q not found in %s", modelID, shared.ModelsDir())
}

func ValidModelNames() []string {
	entries := models.ByFamily(models.FamilyASR)
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.ID)
	}
	return out
}
