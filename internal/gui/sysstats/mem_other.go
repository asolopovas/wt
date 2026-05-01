//go:build !windows && !linux && !android

package sysstats

func MemUsageMB() (usedMB, totalMB int) { return -1, -1 }
