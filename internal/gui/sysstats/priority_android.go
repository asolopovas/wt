//go:build android

package sysstats

import (
	"os"
	"strconv"
	"syscall"
	"time"
)

const (
	prioProcess     = 0
	bgNice          = 10
	bgCpusetTasks   = "/dev/cpuset/background/tasks"
	sysBgCpusetTask = "/dev/cpuset/system-background/tasks"
)

type ThreadPriority struct {
	tid       int
	prevNice  int
	hadCpuset bool
	prevSet   string
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
	if data, rerr := os.ReadFile("/proc/self/task/" + strconv.Itoa(tid) + "/cpuset"); rerr == nil {
		tp.prevSet = string(data)
		tp.hadCpuset = true
	}
	if f, ferr := os.OpenFile(bgCpusetTasks, os.O_WRONLY, 0); ferr == nil {
		_, _ = f.WriteString(strconv.Itoa(tid))
		_ = f.Close()
	} else if f, ferr := os.OpenFile(sysBgCpusetTask, os.O_WRONLY, 0); ferr == nil {
		_, _ = f.WriteString(strconv.Itoa(tid))
		_ = f.Close()
	}
	return tp, true
}

func RestoreThreadPriority(tp ThreadPriority) {
	if tp.tid == 0 {
		return
	}
	_ = syscall.Setpriority(prioProcess, tp.tid, tp.prevNice)
}

func snapshotThreadTIDs() map[int]struct{} {
	ents, err := os.ReadDir("/proc/self/task")
	if err != nil {
		return nil
	}
	out := make(map[int]struct{}, len(ents))
	for _, e := range ents {
		if n, perr := strconv.Atoi(e.Name()); perr == nil {
			out[n] = struct{}{}
		}
	}
	return out
}

func PinNewThreadsBackground(stop <-chan struct{}, baselineSelf int) {
	baseline := snapshotThreadTIDs()
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
				if _, old := baseline[tid]; old {
					continue
				}
				_ = syscall.Setpriority(prioProcess, tid, bgNice)
			}
		}
	}
}
