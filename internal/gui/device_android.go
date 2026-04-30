//go:build android

package gui

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

type deviceStat struct {
	Label string
	Value string
}

func deviceStats() []deviceStat {
	return []deviceStat{
		{"CPU", fmt.Sprintf("%d cores (%s)", runtime.NumCPU(), runtime.GOARCH)},
		{"RAM", readMemTotal()},
		{"GPU", detectGPU()},
	}
}

func detectGPU() string {
	info := detectDevice()
	if info == "" || info == "CPU ONLY" {
		return "—"
	}
	return info
}

func readMemTotal() string {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return "—"
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return "—"
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return "—"
		}
		return fmt.Sprintf("%.1f GB", float64(kb)/1024.0/1024.0)
	}
	return "—"
}

