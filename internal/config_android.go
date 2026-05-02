//go:build android

package shared

import (
	"os"
	"path/filepath"
	"runtime"
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

	n := avail - 2
	if n > 6 {
		n = 6
	}
	if n < 1 {
		n = 1
	}
	return n
}
