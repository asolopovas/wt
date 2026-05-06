package transcriber

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/asolopovas/wt/internal/diarizer"
	"github.com/asolopovas/wt/internal/transcriber/cache"
)

const (
	defaultChunkSec    = 30.0
	minChunkSec        = 5.0
	maxChunkSec        = 60.0
	boundarySearchSec  = 2.0
	boundaryWindowSec  = 0.2
	boundaryMinAdvance = 0.5
)

func envFloat(name string, lo, hi float64) (float64, bool) {
	v := os.Getenv(name)
	if v == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < lo || f > hi {
		return 0, false
	}
	return f, true
}

func chunkSec() float64 {
	for _, env := range []string{"WT_CHUNK_SEC", "WT_SHERPA_CHUNK_SEC"} {
		if f, ok := envFloat(env, minChunkSec, maxChunkSec); ok {
			return f
		}
	}
	return defaultChunkSec
}

func boundarySnapEnabled() bool {
	return os.Getenv("WT_CHUNK_NO_SNAP") == ""
}

type audioChunk struct {
	StartSec float64
	EndSec   float64
	Samples  []float32
}

func samplesAt(sec float64) int {
	return int(sec * float64(WhisperSampleRate))
}

func snapBoundary(samples []float32, target int) int {
	if !boundarySnapEnabled() {
		return target
	}
	window := samplesAt(boundarySearchSec)
	lo := target - window
	hi := target + window
	if lo < samplesAt(boundaryMinAdvance) {
		lo = samplesAt(boundaryMinAdvance)
	}
	if hi > len(samples) {
		hi = len(samples)
	}
	if lo >= hi {
		return target
	}
	step := samplesAt(boundaryWindowSec)
	if step < 1 {
		step = 1
	}
	bestPos := target
	bestEnergy := -1.0
	for pos := lo; pos+step <= hi; pos += step / 2 {
		var sum float64
		for i := pos; i < pos+step; i++ {
			v := float64(samples[i])
			sum += v * v
		}
		if bestEnergy < 0 || sum < bestEnergy {
			bestEnergy = sum
			bestPos = pos + step/2
		}
	}
	return bestPos
}

func splitChunks(samples []float32, sec float64) []audioChunk {
	if sec <= 0 {
		sec = defaultChunkSec
	}
	stride := samplesAt(sec)
	if stride <= 0 {
		stride = WhisperSampleRate * int(defaultChunkSec)
	}
	out := make([]audioChunk, 0, len(samples)/stride+1)
	off := 0
	for off < len(samples) {
		end := off + stride
		if end >= len(samples) {
			end = len(samples)
		} else {
			end = snapBoundary(samples, end)
		}
		if end <= off {
			end = off + stride
			if end > len(samples) {
				end = len(samples)
			}
		}
		out = append(out, audioChunk{
			StartSec: float64(off) / float64(WhisperSampleRate),
			EndSec:   float64(end) / float64(WhisperSampleRate),
			Samples:  samples[off:end],
		})
		if end == len(samples) {
			break
		}
		off = end
	}
	return out
}

func chunkPlanID(sec float64, total int) string {
	return fmt.Sprintf("v2:sec=%.2f:n=%d", sec, total)
}

type chunkProcessor func(ctx context.Context, samples []float32, chunkDurSec float64) ([]diarizer.TranscriptSegment, error)

func shiftTranscriptSegments(segs []diarizer.TranscriptSegment, startSec, endSec float64) {
	off := time.Duration(startSec * float64(time.Second))
	limit := time.Duration(endSec * float64(time.Second))
	clamp := func(t *time.Duration) {
		*t += off
		if *t < off {
			*t = off
		}
		if *t > limit {
			*t = limit
		}
	}
	for i := range segs {
		clamp(&segs[i].Start)
		clamp(&segs[i].End)
		for t := range segs[i].Tokens {
			clamp(&segs[i].Tokens[t].Start)
			clamp(&segs[i].Tokens[t].End)
		}
	}
}

func legacyResumeIndex(chunks []audioChunk, lastEndMs int64) int {
	if lastEndMs <= 0 {
		return 0
	}
	limit := float64(lastEndMs)/1000.0 + 0.05
	for i, ch := range chunks {
		if ch.EndSec > limit {
			return i
		}
	}
	return len(chunks)
}

func resolveResume(rawKey, planID string, chunks []audioChunk, hooks Hooks, engineName string) ([]diarizer.TranscriptSegment, int, error) {
	if rawKey == "" {
		return nil, 0, nil
	}
	part, ok := cache.LoadPartial(rawKey)
	if !ok {
		return nil, 0, nil
	}
	if part.ChunkPlan != "" && part.ChunkPlan != planID {
		hooks.log("info", fmt.Sprintf("%s: chunk plan changed (%s → %s); discarding partial", engineName, part.ChunkPlan, planID))
		cache.DeletePartial(rawKey)
		return nil, 0, nil
	}
	switch hooks.resume(ResumePrompt{
		ResumeAt: time.Duration(part.LastEndMs) * time.Millisecond,
		Segments: len(part.Segments),
	}) {
	case ResumeYes:
		idx := part.CompletedChunks
		if idx <= 0 {
			idx = legacyResumeIndex(chunks, part.LastEndMs)
		}
		hooks.log("info", fmt.Sprintf("%s: resuming from %s (%d cached segs, %d/%d chunks done)",
			engineName,
			FormatHMS(time.Duration(part.LastEndMs)*time.Millisecond),
			len(part.Segments), idx, len(chunks)))
		return part.Segments, idx, nil
	case ResumeFresh:
		cache.DeletePartial(rawKey)
		hooks.log("info", fmt.Sprintf("%s: discarded partial; starting from beginning", engineName))
		return nil, 0, nil
	case ResumeAbort:
		return nil, 0, ErrAborted
	}
	return nil, 0, nil
}

func savePartialState(rawKey, planID string, segs []diarizer.TranscriptSegment, completed int, audioDurSec float64, hooks Hooks, engineName string) {
	if rawKey == "" || completed <= 0 {
		return
	}
	var lastEnd time.Duration
	for _, s := range segs {
		if s.End > lastEnd {
			lastEnd = s.End
		}
	}
	if audioDurSec > 0 && lastEnd.Seconds()/audioDurSec > 0.95 {
		return
	}
	p := cache.Partial{
		Segments:        segs,
		LastEndMs:       lastEnd.Milliseconds(),
		AudioDurMs:      int64(audioDurSec * 1000),
		CompletedChunks: completed,
		ChunkPlan:       planID,
		SavedAt:         time.Now(),
	}
	if err := cache.SavePartial(rawKey, p); err != nil {
		hooks.log("warn", fmt.Sprintf("%s: could not save partial: %v", engineName, err))
	}
}

func finalizeChunked(rawKey string, merged []diarizer.TranscriptSegment, audioDurSec float64, hooks Hooks) []diarizer.TranscriptSegment {
	before := totalTokenCount(merged)
	deruped := DedupRepeatedNgrams(DeduplicateSegments(merged))
	if removed := before - totalTokenCount(deruped); removed > 0 {
		hooks.log("info", fmt.Sprintf("dropped %d repeated word(s) (whisper hallucination guard)", removed))
	}
	if rawKey == "" {
		return deruped
	}
	if ok, reason := cache.RawCacheSafe(deruped, audioDurSec, false); ok {
		if err := cache.SaveRawSegments(rawKey, deruped); err != nil {
			hooks.log("warn", fmt.Sprintf("could not save raw transcript cache: %v", err))
		}
		cache.DeletePartial(rawKey)
	} else {
		hooks.log("debug", "skipped raw cache save: "+reason)
	}
	return deruped
}

func totalTokenCount(segs []diarizer.TranscriptSegment) int {
	n := 0
	for _, s := range segs {
		if len(s.Tokens) > 0 {
			n += len(s.Tokens)
		} else {
			n += len(strings.Fields(s.Text))
		}
	}
	return n
}

func runChunked(
	ctx context.Context,
	engineName string,
	hooks Hooks,
	samples []float32,
	audioDurSec float64,
	rawKey string,
	process chunkProcessor,
) ([]diarizer.TranscriptSegment, float64, error) {
	sec := chunkSec()
	chunks := splitChunks(samples, sec)
	if len(chunks) == 0 {
		return nil, 0, fmt.Errorf("%s: empty input audio", engineName)
	}
	planID := chunkPlanID(sec, len(chunks))

	resumeSegs, resumeIdx, rerr := resolveResume(rawKey, planID, chunks, hooks, engineName)
	if rerr != nil {
		return nil, 0, rerr
	}

	hooks.phase(PhaseTranscribing)
	hooks.progress(PhaseTranscribing, 0)

	merged := append([]diarizer.TranscriptSegment(nil), resumeSegs...)
	var totalElapsed, totalAudio float64

	for i, ch := range chunks {
		if i < resumeIdx {
			continue
		}
		if cerr := ctx.Err(); cerr != nil {
			savePartialState(rawKey, planID, merged, i, audioDurSec, hooks, engineName)
			return nil, 0, ErrAborted
		}

		hooks.log("debug", fmt.Sprintf("%s: chunk %d/%d %s–%s",
			engineName, i+1, len(chunks),
			FormatHMS(time.Duration(ch.StartSec*float64(time.Second))),
			FormatHMS(time.Duration(ch.EndSec*float64(time.Second)))))

		chunkDur := ch.EndSec - ch.StartSec
		start := time.Now()
		chunkSegs, perr := process(ctx, ch.Samples, chunkDur)
		elapsed := time.Since(start).Seconds()
		if perr != nil {
			if errors.Is(perr, ErrAborted) || errors.Is(ctx.Err(), context.Canceled) {
				savePartialState(rawKey, planID, merged, i, audioDurSec, hooks, engineName)
				return nil, 0, ErrAborted
			}
			return nil, 0, fmt.Errorf("%s chunk %d/%d: %w", engineName, i+1, len(chunks), perr)
		}

		shiftTranscriptSegments(chunkSegs, ch.StartSec, ch.EndSec)
		merged = append(merged, chunkSegs...)

		totalElapsed += elapsed
		totalAudio += chunkDur

		savePartialState(rawKey, planID, merged, i+1, audioDurSec, hooks, engineName)

		pct := 100.0 * ch.EndSec / audioDurSec
		if pct > 100 {
			pct = 100
		}
		hooks.progress(PhaseTranscribing, pct)
	}

	hooks.progress(PhaseTranscribing, 100)

	var rtf float64
	if totalElapsed > 0 {
		rtf = totalAudio / totalElapsed
	}
	hooks.log("info", fmt.Sprintf("%s: %d chunk(s) in %.1fs RTF=%.2f",
		engineName, len(chunks), totalElapsed, rtf))

	return finalizeChunked(rawKey, merged, audioDurSec, hooks), rtf, nil
}
