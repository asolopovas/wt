//go:build android

package gui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func queryAndroidGPU() int {
	candidates := []string{
		"/sys/class/kgsl/kgsl-3d0/gpu_busy_percentage",
		"/sys/class/kgsl/kgsl-3d0/devfreq/gpu_load",
	}
	for _, p := range candidates {
		if v := readPercentFile(p); v >= 0 {
			return v
		}
	}
	if matches, _ := filepath.Glob("/sys/class/devfreq/*.mali/load"); len(matches) > 0 {
		if v := readPercentFile(matches[0]); v >= 0 {
			return v
		}
	}
	if matches, _ := filepath.Glob("/sys/class/devfreq/*gpu*/load"); len(matches) > 0 {
		if v := readPercentFile(matches[0]); v >= 0 {
			return v
		}
	}
	if matches, _ := filepath.Glob("/sys/class/devfreq/*gpu*/utilization"); len(matches) > 0 {
		if v := readPercentFile(matches[0]); v >= 0 {
			return v
		}
	}
	return -1
}

func readPercentFile(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return -1
	}
	s := strings.TrimSpace(string(data))
	s = strings.TrimSuffix(s, "%")
	fields := strings.Fields(s)
	if len(fields) > 0 {
		s = fields[0]
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	if v < 0 {
		return -1
	}
	if v > 100 {
		v = 100
	}
	return v
}
