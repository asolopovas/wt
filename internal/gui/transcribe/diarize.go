package transcribe

import (
	"context"
	"fmt"
	"time"

	"github.com/asolopovas/wt/internal/diarizer"
)

func (p *Panel) runDiarization(wavPath string, speakers int, audioDurSec float64) ([]diarizer.Segment, bool) {
	progByName := map[string]func(int64, int64){}
	modelProgress := func(name string, downloaded, total int64) {
		cb, ok := progByName[name]
		if !ok {
			label := map[string]string{"seg": "segmentation", "emb": "embedding"}[name]
			if label == "" {
				label = name
			}
			cb = p.makeDownloadProgress(label)
			progByName[name] = cb
		}
		cb(downloaded, total)
	}
	if err := diarizer.EnsureSherpaModels(modelProgress); err != nil {
		p.AppendLog(fmt.Sprintf("  Diarization model download failed: %v", err))
		return nil, false
	}

	dia, err := diarizer.NewWithPreference(speakers, speakers > 0)
	if err != nil {
		p.AppendLog(fmt.Sprintf("  Diarization unavailable: %v", err))
		return nil, false
	}
	p.debugLog(fmt.Sprintf("diarizer=%s speakers=%d wavPath=%s", dia.Name(), speakers, wavPath))

	ctx, cancel := context.WithCancel(context.Background())
	p.mu.Lock()
	p.cancelFunc = cancel
	p.mu.Unlock()

	diarStart := time.Now()
	lastPct := 0.0

	prog := func(pct float64) {
		if pct <= lastPct {
			return
		}
		lastPct = pct
		p.setStatus(fmt.Sprintf("Diarizing... %.0f%%", pct))
		p.setLocalProgress(0.80 + pct/100.0*0.20)
	}

	diarSegs, err := dia.Diarize(ctx, wavPath, speakers, audioDurSec, prog)
	if err != nil {
		if p.cancelled.Load() {
			return nil, false
		}
		p.AppendLog(fmt.Sprintf("  Diarization failed: %v", err))
		return nil, false
	}

	seen := make(map[int]struct{})
	for _, s := range diarSegs {
		seen[s.Speaker] = struct{}{}
	}
	p.debugLog(fmt.Sprintf("diarized: %d speakers, %d segments, %.0fs",
		len(seen), len(diarSegs), time.Since(diarStart).Seconds()))
	return diarSegs, true
}
