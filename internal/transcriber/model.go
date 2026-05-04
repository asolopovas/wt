package transcriber

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	shared "github.com/asolopovas/wt/internal"
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

type DownloadProgress func(stage string, downloaded, total int64)

func ResolveModelPathWithProgress(modelID, override string, _ DownloadProgress) (string, error) {
	return ResolveModelPathLocal(modelID, override)
}

func ModelDir(modelID string) string {
	return filepath.Join(shared.ModelsDir(), modelID)
}

var ModelFiles = map[string]string{
	"sherpa-whisper-tiny.en":           "sherpa-whisper-tiny.en",
	"sherpa-whisper-tiny":              "sherpa-whisper-tiny",
	"sherpa-whisper-base.en":           "sherpa-whisper-base.en",
	"sherpa-whisper-medium.en":         "sherpa-whisper-medium.en",
	"sherpa-whisper-turbo":             "sherpa-whisper-turbo",
	"moonshine-tiny-en-int8":           "sherpa-onnx-moonshine-tiny-en-int8",
	"parakeet-tdt-0.6b-v2-int8":        "sherpa-onnx-nemo-parakeet-tdt-0.6b-v2-int8",
	"parakeet-tdt-0.6b-v3-int8":        "sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8",
	"sense-voice-zh-en-ja-ko-yue-int8": "sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17",
	"canary-180m-flash":                "sherpa-onnx-nemo-canary-180m-flash",
	"gigaam-v3-ru":                     "sherpa-onnx-nemo-ctc-giga-am-v3",
}

func DefaultModelID() string { return "sherpa-whisper-turbo" }

func ValidModelNames() []string {
	out := make([]string, 0, len(ModelFiles))
	for k := range ModelFiles {
		out = append(out, k)
	}
	return out
}

func ModelDisplayName(modelID string) string {
	if v := strings.TrimPrefix(modelID, "sherpa-"); v != "" {
		return v
	}
	return modelID
}
