package gui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	shared "github.com/asolopovas/wt/internal"
)

func (p *transcribePanel) startStats() {
	p.setStats(p.collectStats())
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			p.setStats(p.collectStats())
		}
	}()
}

func (p *transcribePanel) collectStats() string {
	var parts []string

	if cpu := queryCPUUsage(); cpu >= 0 {
		parts = append(parts, fmt.Sprintf("CPU: %d%%", cpu))
	}

	gpuUtil, gpuMem := queryGpuStats()
	if gpuUtil >= 0 {
		parts = append(parts, fmt.Sprintf("GPU: %d%%", gpuUtil))
	}
	if gpuMem >= 0 {
		parts = append(parts, fmt.Sprintf("VRAM: %dMB", gpuMem))
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	parts = append(parts, fmt.Sprintf("MEM: %dMB", memStats.Sys/1024/1024))

	return strings.Join(parts, "  |  ")
}

func queryGpuStats() (utilPct int, memMB int) {
	utilPct = -1
	memMB = -1

	cmd := exec.Command("nvidia-smi",
		"--query-gpu=utilization.gpu,memory.used",
		"--format=csv,noheader,nounits")
	shared.HideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return
	}

	line := strings.TrimSpace(string(out))
	parts := strings.Split(line, ",")
	if len(parts) < 2 {
		return
	}

	if v, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
		utilPct = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
		memMB = v
	}
	return
}
