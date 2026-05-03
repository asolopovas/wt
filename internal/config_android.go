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
)

// MediaDir resolves the canonical Android shared-storage root for the
// app's recordings, logs, and other user-visible data.
//
// Single home: /storage/emulated/0/Documents/WTranscribe
//   - Visible in any Files app under "Documents → WTranscribe"
//   - Survives uninstall + Clear Data (MANAGE_EXTERNAL_STORAGE granted)
//   - Co-located with Models/ subfolder for easy USB backup
//
// WT_MEDIA_DIR env overrides for tests / power users. If the public
// path is unwritable (permission revoked, etc.) we fall back to the
// app's private CacheDir/imports.
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

// modelsDirOverride probes the canonical public location
// /storage/emulated/0/Documents/WTranscribe/Models. If the app has been
// granted MANAGE_EXTERNAL_STORAGE (declared in our manifest) and the
// directory is writable, prefer it over the private internal-storage
// fallback. Models stored here SURVIVE uninstall + "Clear Data", which
// is the whole point on a phone where the active model set is 4+ GB.
//
// Result is cached for the process lifetime.
// platformModelsDirOverride is consumed by config.go's ModelsDir().
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
		// No public access (permission denied or revoked) — caller will
		// fall back to the private path via the default config.go
		// ModelsDir(). modelsDirPath stays empty so callers can detect
		// the non-override case.
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
