package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

func detectDevice() string {
	var parts []string

	whisper.SetLogQuiet(true)
	if exePath, err := os.Executable(); err == nil {
		whisper.BackendSetSearchPath(filepath.Dir(exePath))
	}
	whisper.BackendLoadAll()
	devices := whisper.BackendDevices()
	for _, dev := range devices {
		if dev.Type == "GPU" || dev.Type == "iGPU" {
			info := dev.Description
			if dev.TotalMB > 0 {
				info += fmt.Sprintf(" (%.1f GB)", float64(dev.TotalMB)/1024.0)
			}
			parts = append(parts, info)
		}
	}

	if len(parts) == 0 {
		return "CPU ONLY"
	}
	return strings.Join(parts, " | ")
}
