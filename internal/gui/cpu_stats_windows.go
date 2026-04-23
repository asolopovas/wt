//go:build windows

package gui

import (
	"syscall"
	"unsafe"
)

var (
	cpuKernel32        = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemTimes = cpuKernel32.NewProc("GetSystemTimes")
)

type cpuFileTime struct {
	Low, High uint32
}

func (f cpuFileTime) uint64() uint64 {
	return uint64(f.High)<<32 | uint64(f.Low)
}

var (
	cpuLastIdle  uint64
	cpuLastTotal uint64
	cpuWarmed    bool
)

func queryCPUUsage() int {
	var idle, kernel, user cpuFileTime
	r1, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if r1 == 0 {
		return -1
	}
	idleT := idle.uint64()
	totalT := kernel.uint64() + user.uint64()

	prevIdle, prevTotal, warmed := cpuLastIdle, cpuLastTotal, cpuWarmed
	cpuLastIdle = idleT
	cpuLastTotal = totalT
	cpuWarmed = true

	if !warmed {
		return -1
	}
	dIdle := idleT - prevIdle
	dTotal := totalT - prevTotal
	if dTotal == 0 {
		return -1
	}
	usage := 100 - int(dIdle*100/dTotal)
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	return usage
}
