//go:build !android

package sysstats

type ThreadPriority struct{}

func SetCurrentThreadBackground() (ThreadPriority, bool) { return ThreadPriority{}, false }
func RestoreThreadPriority(tp ThreadPriority)            {}
func PinNewThreadsBackground(stop <-chan struct{}, baselineSelf int) {
	<-stop
}
