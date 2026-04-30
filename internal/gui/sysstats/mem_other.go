//go:build !windows && !linux

package sysstats

func MemUsageMB() (usedMB, totalMB int) { return -1, -1 }
