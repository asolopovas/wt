//go:build linux

package gui

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var (
	procLastCPU   uint64
	procLastClock time.Time
	procWarmed    bool
)

func clockTicksPerSec() float64 { return 100.0 }

func queryProcessCPU() int {
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return -1
	}
	s := string(data)
	idx := strings.LastIndex(s, ")")
	if idx < 0 || idx+2 >= len(s) {
		return -1
	}
	fields := strings.Fields(s[idx+2:])
	if len(fields) < 13 {
		return -1
	}
	utime, err1 := strconv.ParseUint(fields[11], 10, 64)
	stime, err2 := strconv.ParseUint(fields[12], 10, 64)
	if err1 != nil || err2 != nil {
		return -1
	}
	total := utime + stime
	now := time.Now()

	prev, prevTime, warmed := procLastCPU, procLastClock, procWarmed
	procLastCPU = total
	procLastClock = now
	procWarmed = true

	if !warmed {
		return -1
	}
	dWall := now.Sub(prevTime).Seconds()
	if dWall <= 0 {
		return -1
	}
	dCPU := float64(total-prev) / clockTicksPerSec()
	pct := int(dCPU / dWall / float64(runtime.NumCPU()) * 100)
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return pct
}

func queryProcessRSSMB() int {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return -1
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return -1
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return -1
		}
		return int(kb / 1024)
	}
	return -1
}
