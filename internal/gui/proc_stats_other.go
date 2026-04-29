//go:build !linux

package gui

import "runtime"

func queryProcessCPU() int { return -1 }

func queryProcessRSSMB() int {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int(m.Sys / 1024 / 1024)
}
