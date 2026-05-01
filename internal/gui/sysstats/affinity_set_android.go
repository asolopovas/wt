//go:build android

package sysstats

import (
	"syscall"
	"unsafe"
)

// ReserveTopCores masks off the top `reserve` CPU cores for the calling OS
// thread. Threads spawned afterwards inherit this affinity on Linux, so a
// caller that runs runtime.LockOSThread() before calling this and starts the
// compute pipeline immediately after will see worker threads pinned to the
// remaining cores. Returns the saved mask so it can be restored.
//
// Used to keep the prime + big cluster cores idle during whisper.cpp inference
// so that the UI thread + Fyne render thread aren't fighting for them.
func ReserveTopCores(reserve int) ([]byte, bool) {
	const setSize = 128
	var current [setSize]byte
	r1, _, errno := syscall.RawSyscall(syscall.SYS_SCHED_GETAFFINITY, 0, uintptr(setSize), uintptr(unsafe.Pointer(&current[0])))
	if errno != 0 {
		return nil, false
	}
	used := int(r1)
	if used <= 0 || used > setSize {
		used = setSize
	}
	cores := make([]int, 0, 16)
	for i := 0; i < used; i++ {
		b := current[i]
		for bit := 0; bit < 8; bit++ {
			if b&(1<<bit) != 0 {
				cores = append(cores, i*8+bit)
			}
		}
	}
	if len(cores) <= reserve+1 {
		return nil, false
	}
	saved := make([]byte, used)
	copy(saved, current[:used])

	var newMask [setSize]byte
	for _, c := range cores[:len(cores)-reserve] {
		newMask[c/8] |= 1 << uint(c%8)
	}
	_, _, errno = syscall.RawSyscall(syscall.SYS_SCHED_SETAFFINITY, 0, uintptr(setSize), uintptr(unsafe.Pointer(&newMask[0])))
	if errno != 0 {
		return nil, false
	}
	return saved, true
}

// RestoreAffinity re-applies a mask saved by ReserveTopCores.
func RestoreAffinity(saved []byte) {
	if len(saved) == 0 {
		return
	}
	_, _, _ = syscall.RawSyscall(syscall.SYS_SCHED_SETAFFINITY, 0, uintptr(len(saved)), uintptr(unsafe.Pointer(&saved[0])))
}
