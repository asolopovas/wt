package diarizer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	shared "github.com/asolopovas/wt/internal"
)

type Segment struct {
	Speaker  int
	StartSec float64
	EndSec   float64
}

type ProgressFunc func(pct float64)

type Backend interface {
	Name() string
	Diarize(ctx context.Context, wavPath string, numSpeakers int, audioDurSec float64, progress ProgressFunc) ([]Segment, error)
}

func New() (Backend, error) {
	// Android has no bundled CPython, so it can only use sherpa-onnx.
	if runtime.GOOS == "android" {
		b, err := newSherpaDiarizer()
		if err != nil {
			return nil, fmt.Errorf("sherpa-onnx diarizer unavailable: %w", err)
		}
		return b, nil
	}
	// Desktop: the bundled NeMo (sortformer) Python pipeline gives
	// noticeably better speaker separation than the sherpa-onnx pyannote
	// + 3dspeaker combo on English audio, so prefer it. Fall back to
	// sherpa only if Python / the script aren't available.
	if b, err := newNemoDiarizer(); err == nil {
		return b, nil
	}
	return newSherpaDiarizer()
}

func resolvePython() (string, error) {
	pythonExe := shared.PythonExe()
	if _, err := os.Stat(pythonExe); err != nil {
		if p, lookErr := exec.LookPath("python"); lookErr == nil {
			return p, nil
		}
		return "", fmt.Errorf("python not found at %s", pythonExe)
	}
	return pythonExe, nil
}

func findScript(name string) (string, error) {
	exePath, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exePath), name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	candidate := filepath.Join(shared.Dir(), name)
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}

	cwd, err := os.Getwd()
	if err == nil {
		candidate := filepath.Join(cwd, "scripts", name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("%s not found (should be next to the binary or in %s)", name, shared.Dir())
}
