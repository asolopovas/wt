package transcribe

import (
	"fmt"
	"strings"
	"time"

	"github.com/asolopovas/wt/internal/gui/sysstats"
)

func (p *Panel) startStats() {
	p.setStats(p.collectStats())
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			p.setStats(p.collectStats())
		}
	}()
}

func (p *Panel) collectStats() string {
	var parts []string

	cpu := sysstats.ProcessCPU()
	if cpu < 0 {
		cpu = sysstats.CPUUsage()
	}
	if cpu >= 0 {
		parts = append(parts, fmt.Sprintf("CPU %d%%", cpu))
	}

	gpuUtil, gpuMem := sysstats.GPUStats()
	if gpuUtil >= 0 {
		parts = append(parts, fmt.Sprintf("GPU %d%%", gpuUtil))
	}
	if freq := sysstats.AndroidGPUFreqMHz(); freq > 0 {
		parts = append(parts, fmt.Sprintf("%d MHz", freq))
	}
	if temp := sysstats.AndroidGPUTempC(); temp > 0 {
		parts = append(parts, fmt.Sprintf("%d°C", temp))
	}
	if gpuMem < 0 {
		gpuMem = sysstats.AndroidGPUMemMB()
	}
	if gpuMem >= 0 {
		parts = append(parts, "VRAM "+sysstats.FormatMB(int64(gpuMem)))
	}

	if rss := sysstats.ProcessRSSMB(); rss >= 0 {
		parts = append(parts, "RAM "+sysstats.FormatMB(int64(rss)))
	}

	return strings.Join(parts, "  •  ")
}
