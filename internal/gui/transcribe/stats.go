package transcribe

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"

	"github.com/asolopovas/wt/internal/gui/assets"
	"github.com/asolopovas/wt/internal/gui/sysstats"
)

type statSegment struct {
	icon *fyne.StaticResource
	text string
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

func (p *Panel) collectStats() []statSegment {
	segs := make([]statSegment, 0, 3)

	cpu := sysstats.CPUUsage()
	if cpu < 0 {
		cpu = sysstats.ProcessCPU()
	}
	if cpu >= 0 {
		segs = append(segs, statSegment{icon: assets.CPUIcon, text: fmt.Sprintf("%d%%", cpu)})
	}

	if used, total := sysstats.MemUsageMB(); used >= 0 && total > 0 {
		segs = append(segs, statSegment{
			icon: assets.RAMIcon,
			text: fmt.Sprintf("%s / %s", sysstats.FormatMB(int64(used)), sysstats.FormatMB(int64(total))),
		})
	}

	gpuUtil, gpuUsed, gpuTotal := sysstats.GPUStatsFull()
	if gpuUsed < 0 {
		gpuUsed = sysstats.AndroidGPUMemMB()
	}
	if gpuUsed >= 0 && gpuTotal > 0 {
		segs = append(segs, statSegment{
			icon: assets.GPUIcon,
			text: fmt.Sprintf("%s / %s", sysstats.FormatMB(int64(gpuUsed)), sysstats.FormatMB(int64(gpuTotal))),
		})
	} else if gpuUtil >= 0 {
		segs = append(segs, statSegment{icon: assets.GPUIcon, text: fmt.Sprintf("%d%%", gpuUtil)})
	}

	return segs
}
