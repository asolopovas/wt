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

func queryAndroidGPUFreqMHz() int {
	candidates := []string{
		"/sys/class/kgsl/kgsl-3d0/gpuclk",
		"/sys/class/kgsl/kgsl-3d0/devfreq/cur_freq",
	}
	for _, p := range candidates {
		if v := readIntFile(p); v > 0 {
			return hzToMHz(v)
		}
	}
	for _, pat := range []string{"/sys/class/devfreq/*.mali/cur_freq", "/sys/class/devfreq/*gpu*/cur_freq"} {
		if matches, _ := filepath.Glob(pat); len(matches) > 0 {
			if v := readIntFile(matches[0]); v > 0 {
				return hzToMHz(v)
			}
		}
	}
	return -1
}

func queryAndroidGPUTempC() int {
	matches, _ := filepath.Glob("/sys/class/thermal/thermal_zone*")
	for _, zone := range matches {
		typ, err := os.ReadFile(filepath.Join(zone, "type"))
		if err != nil {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(string(typ)))
		if !strings.Contains(name, "gpu") {
			continue
		}
		if v := readIntFile(filepath.Join(zone, "temp")); v > 0 {
			if v > 1000 {
				v /= 1000
			}
			if v > 0 && v < 200 {
				return v
			}
		}
	}
	return -1
}

func queryAndroidGPUMemMB() int {
	candidates := []string{
		"/sys/class/kgsl/kgsl-3d0/page_alloc_kb",
		"/sys/class/kgsl/kgsl-3d0/page_alloc",
	}
	for _, p := range candidates {
		if v := readIntFile(p); v > 0 {
			if strings.HasSuffix(p, "_kb") {
				return v / 1024
			}
			return v / (1024 * 1024)
		}
	}
	return -1
}

func hzToMHz(v int) int {
	if v >= 1_000_000 {
		return v / 1_000_000
	}
	if v >= 1000 {
		return v / 1000
	}
	return v
}

func readIntFile(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return -1
	}
	s := strings.TrimSpace(string(data))
	fields := strings.Fields(s)
	if len(fields) > 0 {
		s = fields[0]
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return v
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
