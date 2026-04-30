//go:build !windows

package sysstats

func CPUUsage() int { return -1 }
