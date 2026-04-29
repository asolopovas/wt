package gui

import (
	"fmt"
	"os/exec"
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

	cpu := queryProcessCPU()
	if cpu < 0 {
		cpu = queryCPUUsage()
	}
	if cpu >= 0 {
		parts = append(parts, fmt.Sprintf("CPU %d%%", cpu))
	}

	gpuUtil, gpuMem := queryGpuStats()
	if gpuUtil >= 0 {
		parts = append(parts, fmt.Sprintf("GPU %d%%", gpuUtil))
	}
	if freq := queryAndroidGPUFreqMHz(); freq > 0 {
		parts = append(parts, fmt.Sprintf("%d MHz", freq))
	}
	if temp := queryAndroidGPUTempC(); temp > 0 {
		parts = append(parts, fmt.Sprintf("%d°C", temp))
	}
	if gpuMem < 0 {
		gpuMem = queryAndroidGPUMemMB()
	}
	if gpuMem >= 0 {
		parts = append(parts, "VRAM "+formatMB(int64(gpuMem)))
	}

	if rss := queryProcessRSSMB(); rss >= 0 {
		parts = append(parts, "RAM "+formatMB(int64(rss)))
	}

	return strings.Join(parts, "  •  ")
}

func formatMB(mb int64) string {
	if mb >= 1024 {
		return fmt.Sprintf("%.1f GB", float64(mb)/1024)
	}
	return fmt.Sprintf("%d MB", mb)
}

func queryGpuStats() (utilPct int, memMB int) {
	utilPct = -1
	memMB = -1

	if u := queryAndroidGPU(); u >= 0 {
		utilPct = u
		return
	}

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
