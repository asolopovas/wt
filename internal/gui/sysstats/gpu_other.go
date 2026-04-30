//go:build !android

package sysstats

func AndroidGPU() int        { return -1 }
func AndroidGPUFreqMHz() int { return -1 }
func AndroidGPUTempC() int   { return -1 }
func AndroidGPUMemMB() int   { return -1 }
