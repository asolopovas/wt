//go:build android

package sysstats

import (
	"os"
	"strconv"
	"syscall"
	"time"
)

const (
	prioProcess = 0
	bgNice      = 5
)

func setWorkerAffinityMask(saved []byte) {}
func clearWorkerAffinityMask()           {}

func demoteThreadToBackground(tid int) {
	_ = syscall.Setpriority(prioProcess, tid, bgNice)
}

type ThreadPriority struct {
	tid      int
	prevNice int
}

func SetCurrentThreadBackground() (ThreadPriority, bool) {
	tid := syscall.Gettid()
	prev, err := syscall.Getpriority(prioProcess, tid)
	if err != nil {
		return ThreadPriority{}, false
	}
	if errno := syscall.Setpriority(prioProcess, tid, bgNice); errno != nil {
		return ThreadPriority{}, false
	}
	tp := ThreadPriority{tid: tid, prevNice: prev}
	demoteThreadToBackground(tid)
	return tp, true
}

func RestoreThreadPriority(tp ThreadPriority) {
	if tp.tid == 0 {
		return
	}
	_ = syscall.Setpriority(prioProcess, tp.tid, tp.prevNice)
}

var uiThreadNamePrefixes = []string{
	"RenderThread",
	"GLThread",
	"hwuiTask",
	"ANGLE",
	"binder:",
	"HwBinder",
	"Jit thread",
	"Profile Saver",
	"Signal Catcher",
	"FinalizerDaemon",
	"FinalizerWatchd",
	"ReferenceQueueD",
	"HeapTaskDaemon",
	"NDK MediaCodec",
	"CCodecWatchdog",
	"perfetto",
	"android.bg",
	"android.fg",
	"android.ui",
	"android.io",
	"android.display",
	"queued-work",
	"FyneRender",
	"vas.wtranscribe",
}

func isUIOrSystemThread(name string) bool {
	for _, p := range uiThreadNamePrefixes {
		if len(name) >= len(p) && name[:len(p)] == p {
			return true
		}
	}
	return false
}

func PinNewThreadsBackground(stop <-chan struct{}, baselineSelf int) {
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			ents, err := os.ReadDir("/proc/self/task")
			if err != nil {
				continue
			}
			for _, e := range ents {
				tid, perr := strconv.Atoi(e.Name())
				if perr != nil {
					continue
				}
				if tid == baselineSelf {
					continue
				}
				name, rerr := os.ReadFile("/proc/self/task/" + e.Name() + "/comm")
				if rerr != nil {
					continue
				}
				trimmed := string(name)
				if n := len(trimmed); n > 0 && trimmed[n-1] == '\n' {
					trimmed = trimmed[:n-1]
				}
				if isUIOrSystemThread(trimmed) {
					continue
				}
				demoteThreadToBackground(tid)
			}
		}
	}
}
