//go:build !android

package gui

func queryAndroidGPU() int          { return -1 }
func queryAndroidGPUFreqMHz() int   { return -1 }
func queryAndroidGPUTempC() int     { return -1 }
func queryAndroidGPUMemMB() int     { return -1 }
