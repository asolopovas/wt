//go:build windows

package sysstats

import (
	"syscall"
	"unsafe"
)

var (
	memKernel32              = syscall.NewLazyDLL("kernel32.dll")
	procGlobalMemoryStatusEx = memKernel32.NewProc("GlobalMemoryStatusEx")
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func MemUsageMB() (usedMB, totalMB int) {
	var ms memoryStatusEx
	ms.Length = uint32(unsafe.Sizeof(ms))
	r1, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms))) //nolint:gosec // Win32 syscall requires unsafe pointer
	if r1 == 0 {
		return -1, -1
	}
	used := ms.TotalPhys - ms.AvailPhys
	return int(used / 1024 / 1024), int(ms.TotalPhys / 1024 / 1024)
}
