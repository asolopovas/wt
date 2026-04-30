//go:build android

package llm

import (
	"os"
	"path/filepath"
	"strings"
)

func androidLibDirs() []string {
	var dirs []string
	if v := os.Getenv("ANDROID_NATIVE_LIBS_DIR"); v != "" {
		dirs = append(dirs, v)
	}
	for _, env := range []string{"LD_LIBRARY_PATH", "LIB_DIR"} {
		v := os.Getenv(env)
		for _, p := range strings.Split(v, ":") {
			if p != "" {
				dirs = append(dirs, p)
			}
		}
	}
	if data, err := os.ReadFile("/proc/self/maps"); err == nil {
		seen := map[string]bool{}
		for _, line := range strings.Split(string(data), "\n") {
			idx := strings.Index(line, "/data/app/")
			if idx < 0 {
				continue
			}
			path := line[idx:]
			if !strings.HasSuffix(path, ".so") {
				continue
			}
			dir := filepath.Dir(path)
			if seen[dir] {
				continue
			}
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	if matches, err := filepath.Glob("/data/app/*/com.asolopovas.wtranscribe-*/lib/arm64"); err == nil {
		dirs = append(dirs, matches...)
	}
	if matches, err := filepath.Glob("/data/app/com.asolopovas.wtranscribe-*/lib/arm64"); err == nil {
		dirs = append(dirs, matches...)
	}
	return dirs
}
