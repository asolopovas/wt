package gui

import (
	"context"
	"fmt"
	"time"

	"github.com/asolopovas/wt/internal/diarizer"
)

func (p *transcribePanel) runDiarization(wavPath string, speakers int, audioDurSec float64) ([]diarizer.Segment, bool) {
	dlStart := time.Now()
	headerLogged := map[string]bool{}
	var lastLogged time.Time
	modelProgress := func(name string, downloaded, total int64) {
		label := map[string]string{"seg": "segmentation", "emb": "embedding"}[name]
		if label == "" {
			label = name
		}
		if downloaded < 0 {
			p.appendLog(fmt.Sprintf("  %s model download interrupted (retry %d) — resuming...", label, -downloaded))
			lastLogged = time.Time{}
			return
		}
		if total <= 0 {
			return
		}
		if !headerLogged[name] {
			p.appendLog(fmt.Sprintf("  Downloading %s model (%.1f MB)...", label, float64(total)/(1024*1024)))
			headerLogged[name] = true
			dlStart = time.Now()
		}
		dlMB := float64(downloaded) / (1024 * 1024)
		totalMB := float64(total) / (1024 * 1024)
		pct := float64(downloaded) / float64(total)
		elapsed := time.Since(dlStart).Seconds()
		var rate float64
		if elapsed > 0 {
			rate = dlMB / elapsed
		}
		p.setProgress(pct)
		p.setStatus(fmt.Sprintf("Downloading %s model: %.1f / %.1f MB (%.0f%%)", label, dlMB, totalMB, pct*100))
		now := time.Now()
		if downloaded == total || now.Sub(lastLogged) >= 2*time.Second {
			p.appendLog(fmt.Sprintf("  %s: %.1f / %.1f MB (%.0f%%, %.1f MB/s)", label, dlMB, totalMB, pct*100, rate))
			lastLogged = now
		}
	}
	if err := diarizer.EnsureSherpaModels(modelProgress); err != nil {
		p.appendLog(fmt.Sprintf("  Diarization model download failed: %v", err))
		return nil, false
	}

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
