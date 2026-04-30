//go:build !android

package gui

func acquireWakeLock() {}
func releaseWakeLock() {}
