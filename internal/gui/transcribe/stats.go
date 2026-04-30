package transcribe

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"

	"github.com/asolopovas/wt/internal/gui/assets"
	"github.com/asolopovas/wt/internal/gui/sysstats"
)

type statSegment struct {
	icon    *fyne.StaticResource
	compact string
	verbose string
}

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

func memPct(used, total int) int {
	if total <= 0 {
		return -1
	}
	pct := used * 100 / total
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return pct
}

func (p *Panel) collectStats() []statSegment {
	segs := make([]statSegment, 0, 3)

	cpu := sysstats.CPUUsage()
	if cpu < 0 {
		cpu = sysstats.ProcessCPU()
	}
	if cpu >= 0 {
		txt := fmt.Sprintf("%d%%", cpu)
		segs = append(segs, statSegment{icon: assets.CPUIcon, compact: txt, verbose: txt})
	}

	if used, total := sysstats.MemUsageMB(); used >= 0 && total > 0 {
		segs = append(segs, statSegment{
			icon:    assets.RAMIcon,
			compact: fmt.Sprintf("%d%%", memPct(used, total)),
			verbose: fmt.Sprintf("%s / %s", sysstats.FormatMB(int64(used)), sysstats.FormatMB(int64(total))),
		})
	}

	gpuUtil, gpuUsed, gpuTotal := sysstats.GPUStatsFull()
	if gpuUsed < 0 {
		gpuUsed = sysstats.AndroidGPUMemMB()
	}
	if gpuUsed >= 0 && gpuTotal > 0 {
		segs = append(segs, statSegment{
			icon:    assets.GPUIcon,
			compact: fmt.Sprintf("%d%%", memPct(gpuUsed, gpuTotal)),
			verbose: fmt.Sprintf("%s / %s", sysstats.FormatMB(int64(gpuUsed)), sysstats.FormatMB(int64(gpuTotal))),
		})
	} else if gpuUtil >= 0 {
		txt := fmt.Sprintf("%d%%", gpuUtil)
		segs = append(segs, statSegment{icon: assets.GPUIcon, compact: txt, verbose: txt})
	}

	return segs
}
