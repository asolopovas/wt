//go:build linux

package sysstats

import (
	"os"
	"strconv"
	"strings"
)

func MemUsageMB() (usedMB, totalMB int) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return -1, -1
	}
	var total, available int64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		v, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			total = v
		case "MemAvailable:":
			available = v
		}
	}
	if total <= 0 {
		return -1, -1
	}
	used := total - available
	if used < 0 {
		used = 0
	}
	return int(used / 1024), int(total / 1024)
}
