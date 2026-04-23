package gui

import (
	"context"
	"fmt"
	"time"

	"github.com/asolopovas/wt/internal/diarizer"
)

func (p *transcribePanel) runDiarization(wavPath string, speakers int, audioDurSec float64) ([]diarizer.Segment, bool) {
	dia, err := diarizer.New()
	if err != nil {
		p.appendLog(fmt.Sprintf("  Diarization unavailable: %v", err))
		return nil, false
	}
	p.appendLog(fmt.Sprintf("  Diarizing [%s]...", dia.Name()))
	p.debugLog(fmt.Sprintf("diarizer=%s speakers=%d wavPath=%s", dia.Name(), speakers, wavPath))

	ctx, cancel := context.WithCancel(context.Background())
	p.mu.Lock()
	p.cancelFunc = cancel
	p.mu.Unlock()

	diarStart := time.Now()
	lastPct := 0.0

	progress := func(pct float64) {
		if pct <= lastPct {
			return
		}
		lastPct = pct
		p.setStatus(fmt.Sprintf("Diarizing... %.0f%%", pct))
		p.setProgress(pct / 100.0)
	}

	diarSegs, err := dia.Diarize(ctx, wavPath, speakers, audioDurSec, progress)
	if err != nil {
		if p.cancelled.Load() {
			return nil, false
		}
		p.appendLog(fmt.Sprintf("  Diarization failed: %v", err))
		return nil, false
	}

	seen := make(map[int]struct{})
	for _, s := range diarSegs {
		seen[s.Speaker] = struct{}{}
	}
	p.appendLog(fmt.Sprintf("  Diarized (%d speakers, %d segments, %.0fs)",
		len(seen), len(diarSegs), time.Since(diarStart).Seconds()))
	return diarSegs, true
}
