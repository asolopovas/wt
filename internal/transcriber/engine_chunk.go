package transcriber

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/asolopovas/wt/internal/diarizer"
	"github.com/asolopovas/wt/internal/transcriber/cache"
)

// ---------------------------------------------------------------------------
// Generic chunked, resumable ASR driver.
// ---------------------------------------------------------------------------
//
// All ASR engines (Whisper + sherpa-onnx variants) run through this driver.
// The audio is split into fixed ≤30 s windows; each engine implements a
// per-chunk processor that returns chunk-local segments. The driver:
//
//   • shifts segment timestamps onto the absolute audio timeline;
//   • persists a partial transcript after every chunk so an OS-level
//     crash, force-kill, or thermal reboot loses at most one chunk's
//     worth of work (the existing transcriber/cache.Partial format is
//     reused unchanged, so resume is interchangeable across engines);
//   • emits per-chunk progress / log lines uniformly;
//   • handles ctx cancellation cleanly between chunks.
//
// Why this matters: sherpa-onnx-offline loads its entire input WAV at
// once and a 22-minute file on Android-arm64 reliably triggers the kernel
// OOM killer (which can reboot the whole device). Whisper.cpp has its
// own internal windowing and is less prone to OOM, but without explicit
// chunking its resume granularity was "all-or-nothing on cancel". With
// the unified driver, every engine resumes at chunk-boundary granularity.
//
// Window size is tuned via WT_CHUNK_SEC (default 30 s; clamped to
// [5, 60]). The legacy WT_SHERPA_CHUNK_SEC name is honored too.

const (
	defaultChunkSec = 30.0
	minChunkSec     = 5.0
	maxChunkSec     = 60.0
)

func chunkSec() float64 {
	for _, env := range []string{"WT_CHUNK_SEC", "WT_SHERPA_CHUNK_SEC"} {
		if v := os.Getenv(env); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil && f >= minChunkSec && f <= maxChunkSec {
				return f
			}
		}
	}
	return defaultChunkSec
}

// audioChunk is one contiguous slice of the input on the absolute audio
// timeline. Samples is a sub-slice of the full input — no copy.
type audioChunk struct {
	StartSec float64
	EndSec   float64
	Samples  []float32
}

func splitChunks(samples []float32, sec float64) []audioChunk {
	if sec <= 0 {
		sec = defaultChunkSec
	}
	n := int(sec * float64(WhisperSampleRate))
	if n <= 0 {
		n = WhisperSampleRate * int(defaultChunkSec)
	}
	out := make([]audioChunk, 0, len(samples)/n+1)
	for off := 0; off < len(samples); off += n {
		end := off + n
		if end > len(samples) {
			end = len(samples)
		}
		out = append(out, audioChunk{
			StartSec: float64(off) / float64(WhisperSampleRate),
			EndSec:   float64(end) / float64(WhisperSampleRate),
			Samples:  samples[off:end],
		})
	}
	return out
}

// chunkProcessor runs a single chunk through an ASR engine and returns
// segments whose timestamps are CHUNK-LOCAL (start at 0). The driver
// handles shifting onto the absolute timeline and progress reporting.
//
// chunkDurSec is provided as a convenience so processors can set a
// fall-back end-time when no token timestamps are available.
type chunkProcessor func(ctx context.Context, samples []float32, chunkDurSec float64) ([]diarizer.TranscriptSegment, error)

// runChunked is the universal ASR driver. engineName is used purely for
// log lines. process is invoked once per chunk. The function returns
// merged + deduplicated segments and the observed RTF (audio_seconds /
// wall_seconds, computed across only the chunks actually executed in
// this invocation — i.e. excluding any prefix loaded from the resume
// cache).
func runChunked(
	ctx context.Context,
	engineName string,
	hooks Hooks,
	samples []float32,
	audioDurSec float64,
	rawKey string,
	process chunkProcessor,
) (segs []diarizer.TranscriptSegment, rtf float64, err error) {
	chunks := splitChunks(samples, chunkSec())
	if len(chunks) == 0 {
		return nil, 0, fmt.Errorf("%s: empty input audio", engineName)
	}

	// --- Resume support --------------------------------------------------
	var (
		resumeSegs  []diarizer.TranscriptSegment
		resumeAtSec float64
	)
	if rawKey != "" {
		if part, ok := cache.LoadPartial(rawKey); ok {
			switch hooks.resume(ResumePrompt{
				ResumeAt: time.Duration(part.LastEndMs) * time.Millisecond,
				Segments: len(part.Segments),
			}) {
			case ResumeYes:
				resumeSegs = part.Segments
				resumeAtSec = float64(part.LastEndMs) / 1000.0
				hooks.log("info", fmt.Sprintf("%s: resuming from %s (%d cached segs)",
					engineName,
					FormatHMS(time.Duration(part.LastEndMs)*time.Millisecond),
					len(resumeSegs)))
			case ResumeFresh:
				cache.DeletePartial(rawKey)
				hooks.log("info", fmt.Sprintf("%s: discarded partial; starting from beginning", engineName))
			case ResumeAbort:
				return nil, 0, ErrAborted
			}
		}
	}

	hooks.phase(PhaseTranscribing)
	hooks.progress(PhaseTranscribing, 0)

	merged := append([]diarizer.TranscriptSegment(nil), resumeSegs...)
	totalElapsed := 0.0
	totalAudio := 0.0

	for i, ch := range chunks {
		// Resume grace window absorbs floating-point boundary jitter.
		if ch.EndSec <= resumeAtSec+0.05 {
			continue
		}
		if cerr := ctx.Err(); cerr != nil {
			savePartialChunked(rawKey, merged, audioDurSec, hooks, engineName)
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
				savePartialChunked(rawKey, merged, audioDurSec, hooks, engineName)
				return nil, 0, ErrAborted
			}
			return nil, 0, fmt.Errorf("%s chunk %d/%d: %w", engineName, i+1, len(chunks), perr)
		}

		// Shift chunk-local timestamps onto the absolute audio timeline.
		// Both segment-level and per-token (word) timestamps must be
		// shifted — BuildTranscript reads token times for word-level
		// speaker assignment, and stale chunk-local times would map every
		// word in chunk N onto the diarizer track at time 0..chunkSec.
		off := time.Duration(ch.StartSec * float64(time.Second))
		chunkEnd := time.Duration(ch.EndSec * float64(time.Second))
		for j := range chunkSegs {
			chunkSegs[j].Start += off
			chunkSegs[j].End += off
			if chunkSegs[j].End > chunkEnd {
				chunkSegs[j].End = chunkEnd
			}
			if chunkSegs[j].Start < off {
				chunkSegs[j].Start = off
			}
			for t := range chunkSegs[j].Tokens {
				chunkSegs[j].Tokens[t].Start += off
				chunkSegs[j].Tokens[t].End += off
				if chunkSegs[j].Tokens[t].End > chunkEnd {
					chunkSegs[j].Tokens[t].End = chunkEnd
				}
				if chunkSegs[j].Tokens[t].Start < off {
					chunkSegs[j].Tokens[t].Start = off
				}
			}
		}
		merged = append(merged, chunkSegs...)

		totalElapsed += elapsed
		totalAudio += chunkDur

		// Persist partial after every chunk so a crash/reboot loses at
		// most one chunk's worth of progress.
		savePartialChunked(rawKey, merged, audioDurSec, hooks, engineName)

		pct := 100.0 * ch.EndSec / audioDurSec
		if pct > 100 {
			pct = 100
		}
		hooks.progress(PhaseTranscribing, pct)
	}

	hooks.progress(PhaseTranscribing, 100)

	if totalElapsed > 0 {
		rtf = totalAudio / totalElapsed
	}
	hooks.log("info", fmt.Sprintf("%s: %d chunk(s) in %.1fs RTF=%.2f",
		engineName, len(chunks), totalElapsed, rtf))

	deduped := DeduplicateSegments(merged)

	if rawKey != "" {
		if ok, reason := cache.RawCacheSafe(deduped, audioDurSec, false); ok {
			if err := cache.SaveRawSegments(rawKey, deduped); err != nil {
				hooks.log("warn", fmt.Sprintf("could not save raw transcript cache: %v", err))
			}
			cache.DeletePartial(rawKey)
		} else {
			hooks.log("debug", "skipped raw cache save: "+reason)
		}
	}

	return deduped, rtf, nil
}

func savePartialChunked(rawKey string, segs []diarizer.TranscriptSegment, audioDurSec float64, hooks Hooks, engineName string) {
	if rawKey == "" || len(segs) == 0 {
		return
	}
	lastEnd := time.Duration(0)
	for _, s := range segs {
		if s.End > lastEnd {
			lastEnd = s.End
		}
	}
	if lastEnd <= 0 {
		return
	}
	if audioDurSec > 0 && lastEnd.Seconds()/audioDurSec > 0.95 {
		return
	}
	p := cache.Partial{
		Segments:   segs,
		LastEndMs:  lastEnd.Milliseconds(),
		AudioDurMs: int64(audioDurSec * 1000),
		SavedAt:    time.Now(),
	}
	if err := cache.SavePartial(rawKey, p); err != nil {
		hooks.log("warn", fmt.Sprintf("%s: could not save partial: %v", engineName, err))
	}
}
