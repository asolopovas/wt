package transcriber

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/diarizer"
	"github.com/asolopovas/wt/internal/transcriber/cache"
	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

type JobSpec struct {
	SourcePath  string
	ModelSize   string
	Language    string
	Threads     int
	Speakers    int
	NoDiarize   bool
	TDRZ        bool
	DeviceLabel string
}

type Job struct {
	Model    whisper.Model
	Diarizer diarizer.Backend
	Hooks    Hooks
}

type Result struct {
	Transcript       *Transcript
	CacheKey         string
	CachePath        string
	DiarizerName     string
	DetectedLanguage string
	Cached           bool
	RTF              float64
}

type Phase string

const (
	PhaseCacheCheck   Phase = "cache_check"
	PhaseLoadingAudio Phase = "loading_audio"
	PhaseTranscribing Phase = "transcribing"
	PhaseDiarizing    Phase = "diarizing"
	PhaseWriting      Phase = "writing"
)

type Progress struct {
	Phase Phase
	Pct   float64
}

type ResumePrompt struct {
	SourceName string
	ResumeAt   time.Duration
	Segments   int
}

type ResumeChoice int

const (
	ResumeFresh ResumeChoice = iota
	ResumeYes
	ResumeAbort
)

type Hooks struct {
	OnPhase    func(Phase)
	OnProgress func(Progress)
	OnLog      func(level, msg string)
	OnResume   func(ResumePrompt) ResumeChoice
}

func (h Hooks) phase(p Phase) {
	if h.OnPhase != nil {
		h.OnPhase(p)
	}
}

func (h Hooks) progress(phase Phase, pct float64) {
	if h.OnProgress != nil {
		h.OnProgress(Progress{Phase: phase, Pct: pct})
	}
}

func (h Hooks) log(level, msg string) {
	if h.OnLog != nil {
		h.OnLog(level, msg)
	}
}

func (h Hooks) resume(p ResumePrompt) ResumeChoice {
	if h.OnResume == nil {
		return ResumeFresh
	}
	return h.OnResume(p)
}

var ErrAborted = errors.New("transcription aborted")

func (j *Job) Run(ctx context.Context, spec JobSpec) (Result, error) {
	if j.Model == nil {
		return Result{}, fmt.Errorf("job: Model is required")
	}
	if spec.SourcePath == "" {
		return Result{}, fmt.Errorf("job: SourcePath is required")
	}

	absPath, err := filepath.Abs(spec.SourcePath)
	if err != nil {
		return Result{}, fmt.Errorf("resolving path: %w", err)
	}
	if _, err := os.Stat(absPath); err != nil {
		return Result{}, fmt.Errorf("file not found: %s", absPath)
	}

	j.Hooks.phase(PhaseCacheCheck)

	keyParams, keyErr := cache.BuildKeyParams(absPath, spec.ModelSize, spec.Language, spec.Speakers, spec.NoDiarize)
	var cacheKey string
	if keyErr == nil {
		cacheKey = cache.ComputeKey(keyParams)
		if hitPath, _, ok := cache.Lookup(cacheKey); ok {
			j.Hooks.log("info", "cache hit; reusing transcript")
			return Result{
				CacheKey:  cacheKey,
				CachePath: hitPath,
				Cached:    true,
			}, nil
		}
	}

	if err := ctx.Err(); err != nil {
		return Result{}, ErrAborted
	}

	j.Hooks.phase(PhaseLoadingAudio)
	loadStart := time.Now()
	samples, err := LoadAudioSamples(absPath)
	if err != nil {
		return Result{}, fmt.Errorf("loading audio: %w", err)
	}
	audioDurSec := float64(len(samples)) / WhisperSampleRate
	j.Hooks.log("debug", fmt.Sprintf("audio loaded (%s, %.1fs) samples=%d",
		FormatHMS(time.Duration(audioDurSec*float64(time.Second))),
		time.Since(loadStart).Seconds(), len(samples)))

	if err := ctx.Err(); err != nil {
		return Result{}, ErrAborted
	}

	var (
		segs       []diarizer.TranscriptSegment
		detectedLang string
		rawKey     string
		rawHit     bool
		rtf        float64
	)
	if keyErr == nil {
		rawKey = cache.ComputeRawKey(keyParams.SourcePath, keyParams.MtimeNs, spec.ModelSize, spec.Language)
		if cached, ok := cache.LoadRawSegments(rawKey); ok {
			segs = cached
			rawHit = true
			detectedLang = spec.Language
			j.Hooks.log("info", fmt.Sprintf("raw transcript reused (%d segs)", len(cached)))
		}
	}

	if !rawHit {
		whisperSegs, dl, observedRTF, err := j.runWhisper(ctx, spec, samples, audioDurSec, rawKey)
		if err != nil {
			return Result{}, err
		}
		segs = whisperSegs
		detectedLang = dl
		rtf = observedRTF
	}
	if detectedLang == "" {
		detectedLang = spec.Language
	}

	if err := ctx.Err(); err != nil {
		return Result{}, ErrAborted
	}

	usedTDRZ := UseTDRZ(spec.TDRZ, false, spec.NoDiarize)

	var (
		diarSegs []diarizer.Segment
		diarOK   bool
		diarName string
	)
	if !spec.NoDiarize && diarizer.SupportsExternalBackend() {
		dSegs, dName, ok := j.runDiarize(ctx, absPath, samples, audioDurSec, spec.Speakers)
		diarSegs, diarName, diarOK = dSegs, dName, ok
	}

	if !diarOK && usedTDRZ {
		diarSegs = diarizer.SpeakerTurnSegments(segs)
		if len(diarSegs) > 0 {
			diarOK = true
			diarName = "tinydiarize"
		}
	}

	j.Hooks.phase(PhaseWriting)

	transcript := BuildTranscript(segs, diarSegs, diarOK, TranscriptMeta{
		Model:      spec.ModelSize,
		Language:   detectedLang,
		DurationMs: int64(audioDurSec * 1000),
		Diarizer:   diarName,
		Device:     spec.DeviceLabel,
	})

	if cacheKey == "" {
		cacheKey = cache.ComputeKey(cache.KeyParams{
			SourcePath: absPath,
			MtimeNs:    time.Now().UnixNano(),
			Model:      spec.ModelSize,
			Language:   detectedLang,
			Speakers:   spec.Speakers,
			NoDiarize:  spec.NoDiarize,
		})
	}

	data, err := json.MarshalIndent(transcript, "", "  ")
	if err != nil {
		return Result{}, fmt.Errorf("marshaling transcript: %w", err)
	}
	data = append(data, '\n')

	entry := cache.Entry{
		Key:        cacheKey,
		SourcePath: absPath,
		SourceName: filepath.Base(absPath),
		Model:      spec.ModelSize,
		Language:   detectedLang,
		Speakers:   spec.Speakers,
		NoDiarize:  spec.NoDiarize,
		Utterances: len(transcript.Utterances),
		DurationMs: int64(audioDurSec * 1000),
		CreatedAt:  time.Now(),
	}
	storedPath, storeErr := cache.Store(entry, data)
	if storeErr != nil {
		return Result{}, fmt.Errorf("storing transcript: %w", storeErr)
	}

	return Result{
		Transcript:       transcript,
		CacheKey:         cacheKey,
		CachePath:        storedPath,
		DiarizerName:     diarName,
		DetectedLanguage: detectedLang,
		RTF:              rtf,
	}, nil
}

func (j *Job) runWhisper(ctx context.Context, spec JobSpec, samples []float32, audioDurSec float64, rawKey string) ([]diarizer.TranscriptSegment, string, float64, error) {
	wctx, err := j.Model.NewContext()
	if err != nil {
		return nil, "", 0, fmt.Errorf("creating context: %w", err)
	}

	ConfigureContext(wctx, ContextConfig{
		Threads: spec.Threads,
		TDRZ:    UseTDRZ(spec.TDRZ, false, spec.NoDiarize),
	})
	if ConfigureVAD(wctx) {
		j.Hooks.log("debug", "VAD: Silero v6.2.0")
	}
	SetLanguage(wctx, spec.Language)

	var (
		resumeSegs []diarizer.TranscriptSegment
		offsetMs   int64
	)
	if rawKey != "" {
		if part, ok := cache.LoadPartial(rawKey); ok {
			choice := j.Hooks.resume(ResumePrompt{
				SourceName: filepath.Base(spec.SourcePath),
				ResumeAt:   time.Duration(part.LastEndMs) * time.Millisecond,
				Segments:   len(part.Segments),
			})
			switch choice {
			case ResumeYes:
				resumeSegs = part.Segments
				offsetMs = part.LastEndMs
				wctx.SetOffset(time.Duration(offsetMs) * time.Millisecond)
				j.Hooks.log("info", fmt.Sprintf("resuming from %s (%d cached segs)",
					FormatHMS(time.Duration(offsetMs)*time.Millisecond), len(resumeSegs)))
			case ResumeFresh:
				cache.DeletePartial(rawKey)
				j.Hooks.log("info", "discarded partial transcript; starting from beginning")
			case ResumeAbort:
				return nil, "", 0, ErrAborted
			}
		}
	}

	j.Hooks.phase(PhaseTranscribing)
	processStart := time.Now()
	abortCb := func() bool { return ctx.Err() == nil }
	progressCb := func(pct int) {
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		j.Hooks.progress(PhaseTranscribing, float64(pct))
	}

	procErr := wctx.Process(samples, abortCb, nil, whisper.ProgressCallback(progressCb))

	newSegs := ExtractSegments(wctx)
	merged := make([]diarizer.TranscriptSegment, 0, len(resumeSegs)+len(newSegs))
	merged = append(merged, resumeSegs...)
	merged = append(merged, newSegs...)

	if procErr != nil {
		if ctx.Err() != nil {
			if rawKey != "" {
				j.savePartialIfUseful(rawKey, merged, audioDurSec)
			}
			return nil, "", 0, ErrAborted
		}
		return nil, "", 0, fmt.Errorf("processing audio: %w", procErr)
	}

	elapsed := time.Since(processStart).Seconds()
	rtf := 0.0
	remainSec := audioDurSec - float64(offsetMs)/1000.0
	if elapsed > 0 && remainSec > 0 {
		rtf = remainSec / elapsed
	}
	j.Hooks.log("debug", fmt.Sprintf("transcribed in %.0fs RTF=%.2f", elapsed, rtf))

	detected := wctx.DetectedLanguage()
	if detected == "" {
		detected = spec.Language
	}

	deduped := DeduplicateSegments(merged)
	if dropped := len(merged) - len(deduped); dropped > 0 {
		j.Hooks.log("debug", fmt.Sprintf("dedup: removed %d repeated segments", dropped))
	}

	if rawKey != "" {
		if ok, reason := cache.RawCacheSafe(deduped, audioDurSec, false); ok {
			if err := cache.SaveRawSegments(rawKey, deduped); err != nil {
				j.Hooks.log("warn", fmt.Sprintf("could not save raw transcript cache: %v", err))
			}
			cache.DeletePartial(rawKey)
		} else {
			j.Hooks.log("debug", "skipped raw cache save: "+reason)
		}
	}

	return deduped, detected, rtf, nil
}

func (j *Job) savePartialIfUseful(rawKey string, segs []diarizer.TranscriptSegment, audioDurSec float64) {
	if len(segs) == 0 {
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
		j.Hooks.log("warn", fmt.Sprintf("could not save partial: %v", err))
		return
	}
	j.Hooks.log("info", fmt.Sprintf("saved partial transcript (%s, %d segs)",
		FormatHMS(lastEnd), len(segs)))
}

func (j *Job) runDiarize(ctx context.Context, absPath string, samples []float32, audioDurSec float64, speakers int) ([]diarizer.Segment, string, bool) {
	dia := j.Diarizer
	if dia == nil {
		var err error
		dia, err = diarizer.New(speakers)
		if err != nil {
			j.Hooks.log("warn", fmt.Sprintf("diarization unavailable: %v", err))
			return nil, "", false
		}
	}

	wavPath := j.ensureWAVForDiarize(absPath, samples)

	j.Hooks.phase(PhaseDiarizing)
	diarStart := time.Now()
	lastPct := 0.0
	prog := func(pct float64) {
		if pct <= lastPct {
			return
		}
		lastPct = pct
		j.Hooks.progress(PhaseDiarizing, pct)
	}

	diarSegs, err := dia.Diarize(ctx, wavPath, speakers, audioDurSec, prog)
	if err != nil {
		if ctx.Err() != nil {
			return nil, "", false
		}
		j.Hooks.log("warn", fmt.Sprintf("diarization failed: %v", err))
		return nil, "", false
	}

	seen := make(map[int]struct{})
	for _, s := range diarSegs {
		seen[s.Speaker] = struct{}{}
	}
	j.Hooks.log("debug", fmt.Sprintf("diarized: %s · %d speakers · %d segments · %.0fs",
		dia.Name(), len(seen), len(diarSegs), time.Since(diarStart).Seconds()))
	return diarSegs, dia.Name(), true
}

func (j *Job) ensureWAVForDiarize(absPath string, samples []float32) string {
	wavPath := ResolveWAVPath(absPath)
	if strings.HasSuffix(strings.ToLower(wavPath), ".wav") && wavPath != absPath {
		return wavPath
	}
	audioKey, err := AudioCacheKey(absPath)
	if err != nil {
		return absPath
	}
	cachePath := filepath.Join(shared.CacheDir(), audioKey)
	if _, statErr := os.Stat(cachePath); statErr == nil {
		return cachePath
	}
	if werr := WritePCM16WAV(cachePath, samples, WhisperSampleRate); werr != nil {
		j.Hooks.log("warn", fmt.Sprintf("could not write WAV cache: %v", werr))
		return absPath
	}
	return cachePath
}
