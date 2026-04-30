//go:build !linux

package sysstats

import "runtime"

func ProcessCPU() int { return -1 }

func ProcessRSSMB() int {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int(m.Sys / 1024 / 1024)
}
