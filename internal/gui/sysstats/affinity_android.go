//go:build android

package sysstats

import (
	"syscall"
	"unsafe"
)

func AffinityCPUCount() int {
	const setSize = 128
	var mask [setSize]byte
	r1, _, errno := syscall.RawSyscall(syscall.SYS_SCHED_GETAFFINITY, 0, uintptr(setSize), uintptr(unsafe.Pointer(&mask[0])))
	if errno != 0 {
		return -1
	}
	used := int(r1)
	if used <= 0 || used > setSize {
		used = setSize
	}
	count := 0
	for i := 0; i < used; i++ {
		b := mask[i]
		for b != 0 {
			count += int(b & 1)
			b >>= 1
		}
	}
	if count <= 0 {
		return -1
	}
	return count
}
