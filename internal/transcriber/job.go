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
)

type JobSpec struct {
	SourcePath  string
	ModelSize   string
	Language    string
	Engine      string
	Threads     int
	Speakers    int
	NoDiarize   bool
	DeviceLabel string
}

type Job struct {
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

	var (
		segs         []diarizer.TranscriptSegment
		detectedLang string
		rawKey       string
		rawHit       bool
		rtf          float64
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

	diarizeWanted := !spec.NoDiarize && diarizer.SupportsExternalBackend()
	diarHasReadyWAV := diarizeWanted && diarizeWAVCached(absPath)
	needSamples := !rawHit || (diarizeWanted && !diarHasReadyWAV)
	_ = needSamples

	j.Hooks.phase(PhaseLoadingAudio)
	var (
		samples     []float32
		audioDurSec float64
	)
	_ = strings.EqualFold
	switch {
	case needSamples:
		loadStart := time.Now()
		loaded, err := LoadAudioSamples(absPath)
		if err != nil {
			return Result{}, fmt.Errorf("loading audio: %w", err)
		}
		samples = loaded
		audioDurSec = float64(len(samples)) / WhisperSampleRate
		j.Hooks.log("debug", fmt.Sprintf("audio loaded (%s, %.1fs) samples=%d",
			FormatHMS(time.Duration(audioDurSec*float64(time.Second))),
			time.Since(loadStart).Seconds(), len(samples)))
	default:
		if ms := ProbeDurationMs(absPath); ms > 0 {
			audioDurSec = float64(ms) / 1000.0
		}
		j.Hooks.log("debug", fmt.Sprintf("audio decode skipped (raw cache hit, dur=%s)",
			FormatHMS(time.Duration(audioDurSec*float64(time.Second)))))
	}

	if err := ctx.Err(); err != nil {
		return Result{}, ErrAborted
	}

	if !rawHit {
		asrSegs, dl, observedRTF, err := j.runASR(ctx, spec, samples, audioDurSec, rawKey)
		if err != nil {
			return Result{}, err
		}
		segs = asrSegs
		detectedLang = dl
		rtf = observedRTF
	}
	if detectedLang == "" {
		detectedLang = spec.Language
	}

	if err := ctx.Err(); err != nil {
		return Result{}, ErrAborted
	}

	var (
		diarSegs []diarizer.Segment
		diarOK   bool
		diarName string
	)
	if diarizeWanted {
		dSegs, dName, ok := j.runDiarize(ctx, absPath, samples, audioDurSec, spec.Speakers)
		diarSegs, diarName, diarOK = dSegs, dName, ok
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
	if samples == nil {
		return absPath
	}
	if werr := WritePCM16WAV(cachePath, samples, WhisperSampleRate); werr != nil {
		j.Hooks.log("warn", fmt.Sprintf("could not write WAV cache: %v", werr))
		return absPath
	}
	return cachePath
}

func diarizeWAVCached(absPath string) bool {
	if strings.HasSuffix(strings.ToLower(absPath), ".wav") {
		return true
	}
	key, err := AudioCacheKey(absPath)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(shared.CacheDir(), key))
	return err == nil
}
