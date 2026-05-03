//go:build android

package shared

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

func appDir() string {
	filesDir := os.Getenv("FILESDIR")
	if filesDir != "" {
		return filepath.Join(filesDir, "wt")
	}

	return "/data/data/com.asolopovas.wtranscribe/files/wt"
}

var (
	mediaDirOnce sync.Once
	mediaDirPath string

	modelsDirOnce sync.Once
	modelsDirPath string

	configFileOnce sync.Once
	configFilePath string
)

func platformConfigFileOverride() string {
	configFileOnce.Do(func() {
		if v := os.Getenv("WT_CONFIG_FILE"); v != "" {
			configFilePath = v
			return
		}
		public := "/storage/emulated/0/Documents/WTranscribe"
		if err := os.MkdirAll(public, 0o755); err == nil {
			probe := filepath.Join(public, ".wt-write-test")
			if f, err := os.Create(probe); err == nil {
				_ = f.Close()
				_ = os.Remove(probe)
				configFilePath = filepath.Join(public, "config.yml")
				return
			}
		}

	})
	return configFilePath
}

func MediaDir() string {
	mediaDirOnce.Do(func() {
		if v := os.Getenv("WT_MEDIA_DIR"); v != "" {
			mediaDirPath = v
			return
		}
		public := "/storage/emulated/0/Documents/WTranscribe"
		if err := os.MkdirAll(public, 0o755); err == nil {
			probe := filepath.Join(public, ".wt-write-test")
			if f, err := os.Create(probe); err == nil {
				_ = f.Close()
				_ = os.Remove(probe)
				mediaDirPath = public
				return
			}
		}
		mediaDirPath = filepath.Join(CacheDir(), "imports")
	})
	return mediaDirPath
}

func platformModelsDirOverride() string {
	return modelsDirOverride()
}

func modelsDirOverride() string {
	modelsDirOnce.Do(func() {
		if v := os.Getenv("WT_MODELS_DIR"); v != "" {
			modelsDirPath = v
			return
		}
		public := "/storage/emulated/0/Documents/WTranscribe/Models"
		if err := os.MkdirAll(public, 0o755); err == nil {
			probe := filepath.Join(public, ".wt-write-test")
			if f, err := os.Create(probe); err == nil {
				_ = f.Close()
				_ = os.Remove(probe)
				modelsDirPath = public
				return
			}
		}

	})
	return modelsDirPath
}

func defaultModel() string {
	return "tiny"
}

func affinityCPUs() int {
	const setSize = 128
	var mask [setSize]byte
	r1, _, errno := syscall.RawSyscall(syscall.SYS_SCHED_GETAFFINITY, 0, uintptr(setSize), uintptr(unsafe.Pointer(&mask[0])))
	if errno != 0 {
		return runtime.NumCPU()
	}
	count := 0
	used := int(r1)
	if used <= 0 || used > setSize {
		used = setSize
	}
	for i := 0; i < used; i++ {
		b := mask[i]
		for b != 0 {
			count += int(b & 1)
			b >>= 1
		}
	}
	if count <= 0 {
		return runtime.NumCPU()
	}
	return count
}

func defaultThreads() int {
	avail := affinityCPUs()

	n := avail - 4
	if n > 4 {
		n = 4
	}
	if n < 1 {
		n = 1
	}
	return n
}
