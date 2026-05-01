package sysstats

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

type ProcSnapshot struct {
	PID      int
	RSSMB    int
	Threads  int
	Cpuset   string
	CPUPct   int
	NumCores int
}

func ProcStats() ProcSnapshot {
	snap := ProcSnapshot{
		PID:      os.Getpid(),
		RSSMB:    ProcessRSSMB(),
		Threads:  -1,
		Cpuset:   "",
		CPUPct:   ProcessCPU(),
		NumCores: AffinityCPUCount(),
	}
	if snap.NumCores <= 0 {
		snap.NumCores = runtime.NumCPU()
	}

	if data, err := os.ReadFile("/proc/self/status"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, "Threads:") {
				continue
			}
			f := strings.Fields(line)
			if len(f) >= 2 {
				if v, err := strconv.Atoi(f[1]); err == nil {
					snap.Threads = v
				}
			}
			break
		}
	}

	if data, err := os.ReadFile("/proc/self/cpuset"); err == nil {
		snap.Cpuset = strings.TrimSpace(string(data))
	}
	return snap
}
