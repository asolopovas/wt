//go:build !android

package platsvc

func AcquireWakeLock() {}
func ReleaseWakeLock() {}
