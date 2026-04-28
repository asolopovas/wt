package gui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2/dialog"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/diarizer"
	"github.com/asolopovas/wt/internal/transcriber"
	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

func formatETA(secs float64) string {
	if secs < 0 {
		secs = 0
	}
	if secs < 60 {
		return fmt.Sprintf("%.0fs", secs)
	}
	if secs < 3600 {
		return fmt.Sprintf("%dm%02ds", int(secs)/60, int(secs)%60)
	}
	return fmt.Sprintf("%dh%02dm", int(secs)/3600, (int(secs)%3600)/60)
}

type etaTracker struct {
	start   time.Time
	lastPct int
}

func newETATracker() *etaTracker {
	return &etaTracker{start: time.Now(), lastPct: 0}
}

func (e *etaTracker) update(pct int) string {
	if pct <= 0 {
		return ""
	}
	e.lastPct = pct
	elapsed := time.Since(e.start)
	totalEstimate := elapsed * time.Duration(100) / time.Duration(pct)
	remaining := totalEstimate - elapsed
	if remaining < 0 {
		remaining = 0
	}
	return transcriber.FormatHMS(remaining)
}

func (p *transcribePanel) onTranscribe() {
	p.mu.Lock()
	running := p.running
	p.mu.Unlock()
	if running {
		p.onCancel()
		return
	}

	if len(p.files) == 0 {
		dialog.ShowInformation("No Files", "Drop audio files to transcribe.", p.window)
		return
	}

	p.logText.Segments = nil
	p.logText.Refresh()
	p.cancelled.Store(false)

	go p.runTranscription()
}

func (p *transcribePanel) runTranscription() {
	p.setRunning(true)
	defer p.setRunning(false)

	p.results = nil
	p.resetSpeakerRenames()

	modelSize := p.settings.modelSize()
	device := p.settings.device()
	threads := p.settings.threads()
	language := p.settings.language()
	speakers := p.settings.speakers()
	noDiarize := p.settings.noDiarize()

	p.appendLog(fmt.Sprintf("Loading model: %s (%s)...", modelSize, device))
	p.setStatus("Loading model...")
	p.debugLog(fmt.Sprintf("threads=%d language=%q speakers=%d noDiarize=%v", threads, language, speakers, noDiarize))

	model, err := p.loadModel(modelSize)
	if err != nil {
		p.appendLog(fmt.Sprintf("Error: %v", err))
		p.setStatus("Model loading failed")
		return
	}
	defer func() {
		_ = model.Close()
	}()

	gpuFound := false
	devices := whisper.BackendDevices()
	for _, dev := range devices {
		if dev.Type == "GPU" || dev.Type == "iGPU" {
			gpuFound = true
			p.appendLog(fmt.Sprintf("Model loaded (%s, %s)", modelSize, dev.Description))
			usedMB := dev.TotalMB - dev.FreeMB
			p.appendLog(fmt.Sprintf("VRAM: %d/%d MB", usedMB, dev.TotalMB))
			p.debugLog(fmt.Sprintf("GPU: %s (free=%dMB total=%dMB)", dev.Description, dev.FreeMB, dev.TotalMB))
		}
	}
	if !gpuFound {
		p.appendLog(fmt.Sprintf("Model loaded (%s, CPU)", modelSize))
		p.debugLog("no GPU detected, using CPU")
	}
	p.debugLog(fmt.Sprintf("system: %d cores, %s", runtime.NumCPU(), runtime.GOARCH))

	total := len(p.files)
	errCount := 0

	for i, path := range p.files {
		if p.cancelled.Load() {
			p.appendLog("Cancelled by user.")
			p.setStatus("Cancelled.")
			return
		}

		filename := filepath.Base(path)
		p.setStatus(fmt.Sprintf("[%d/%d] %s", i+1, total, filename))
		p.setProgress(float64(i) / float64(total))
		p.appendLog(fmt.Sprintf("[%d/%d] %s", i+1, total, filename))

		if err := p.transcribeFile(model, path, modelSize, language, threads, speakers, noDiarize); err != nil {
			if p.cancelled.Load() {
				p.appendLog("Cancelled by user.")
				p.setStatus("Cancelled.")
				return
			}
			p.appendLog(fmt.Sprintf("  Error: %v", err))
			errCount++
			continue
		}
	}

	p.setProgress(1.0)

	if errCount > 0 {
		msg := fmt.Sprintf("Done: %d/%d transcribed, %d failed.", total-errCount, total, errCount)
		p.appendLog(msg)
		p.setStatus(msg)
	} else if total == 1 {
		p.appendLog("Transcription complete.")
		p.setStatus("Transcription complete.")
	} else {
		msg := fmt.Sprintf("All %d files transcribed.", total)
		p.appendLog(msg)
		p.setStatus(msg)
	}
}

func (p *transcribePanel) loadModel(modelSize string) (whisper.Model, error) {
	prog := p.makeDownloadProgress(modelSize)

	path, err := transcriber.ResolveModelPathWithProgress(modelSize, "", prog)
	if err != nil {
		return nil, err
	}

	exePath, err := os.Executable()
	if err == nil {
		whisper.BackendSetSearchPath(filepath.Dir(exePath))
	}

	whisper.SetLogQuiet(true)

	start := time.Now()
	m, err := whisper.New(path)
	if err != nil {
		return nil, fmt.Errorf("loading model: %w", err)
	}

	p.appendLog(fmt.Sprintf("  Model loaded in %.1fs", time.Since(start).Seconds()))
	return m, nil
}

func (p *transcribePanel) transcribeFile(model whisper.Model, path, modelSize, language string, threads, speakers int, noDiarize bool) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("file not found: %s", absPath)
	}

	sourceName := filepath.Base(absPath)

	params, keyErr := buildCacheParams(absPath, modelSize, language, speakers, noDiarize)
	var cacheKey string
	if keyErr == nil {
		cacheKey = computeCacheKey(params)
		if hitPath, _, ok := cacheLookup(cacheKey); ok {
			p.lastCSVPath = hitPath
			p.results = append(p.results, exportItem{cachePath: hitPath, sourceName: sourceName})
			p.appendLog(fmt.Sprintf("  Cached transcript reused for %s", sourceName))
			p.setProgress(1.0)
			if p.history != nil {
				p.history.refresh()
			}
			return nil
		}
	}

	p.appendLog("  Loading audio...")
	loadStart := time.Now()
	samples, err := transcriber.LoadAudioSamples(absPath)
	if err != nil {
		return fmt.Errorf("loading audio: %w", err)
	}

	audioDurSec := float64(len(samples)) / transcriber.WhisperSampleRate
	durStr := transcriber.FormatHMS(time.Duration(audioDurSec * float64(time.Second)))
	p.appendLog(fmt.Sprintf("  Audio loaded (%s, %.1fs)", durStr, time.Since(loadStart).Seconds()))
	p.debugLog(fmt.Sprintf("samples=%d sampleRate=%d duration=%.1fs", len(samples), transcriber.WhisperSampleRate, audioDurSec))

	if p.cancelled.Load() {
		return fmt.Errorf("cancelled")
	}

	var diarSegs []diarizer.Segment
	diarOK := false
	if !noDiarize && diarizer.SupportsExternalBackend() {
		wavPath := transcriber.ResolveWAVPath(absPath)
		// Sherpa-onnx requires a real 16kHz mono PCM WAV file. If the cache
		// only has the source m4a/flac, materialize the decoded samples to
		// disk first so the diarizer subprocess can read them.
		if !strings.HasSuffix(strings.ToLower(wavPath), ".wav") || wavPath == absPath {
			cacheKey, err := transcriber.AudioCacheKey(absPath)
			if err == nil {
				cachePath := filepath.Join(shared.CacheDir(), cacheKey)
				if _, statErr := os.Stat(cachePath); statErr != nil {
					if werr := transcriber.WritePCM16WAV(cachePath, samples, transcriber.WhisperSampleRate); werr == nil {
						wavPath = cachePath
					} else {
						p.debugLog(fmt.Sprintf("could not write WAV cache: %v", werr))
					}
				} else {
					wavPath = cachePath
				}
			}
		}
		diarSegs, diarOK = p.runDiarization(wavPath, speakers, audioDurSec)
	}
	usedTDRZ := transcriber.UseTDRZ(false, diarOK, noDiarize)

	if p.cancelled.Load() {
		return fmt.Errorf("cancelled")
	}

	ctx, err := model.NewContext()
	if err != nil {
		return fmt.Errorf("creating context: %w", err)
	}

	transcriber.ConfigureContext(ctx, transcriber.ContextConfig{
		Threads: threads,
		TDRZ:    usedTDRZ,
	})

	p.debugLog(fmt.Sprintf("TDRZ=%v (diarOK=%v)", usedTDRZ, diarOK))

	if transcriber.ConfigureVAD(ctx) {
		p.appendLog("  VAD: Silero v6.2.0")
	}

	transcriber.SetLanguage(ctx, language)

	p.appendLog("  Transcribing...")
	processStart := time.Now()
	lastPct := -1
	eta := newETATracker()

	abortCb := func() bool {
		return !p.cancelled.Load()
	}

	progressCb := func(pct int) {
		if pct > 100 {
			pct = 100
		}
		if pct > lastPct {
			lastPct = pct
			fileProgress := float64(pct) / 100.0
			p.setProgress(fileProgress)
			etaStr := eta.update(pct)
			if etaStr != "" {
				p.setStatus(fmt.Sprintf("Transcribing... %d%%  ETA: %s", pct, etaStr))
			} else {
				p.setStatus(fmt.Sprintf("Transcribing... %d%%", pct))
			}
		}
	}

	if err := ctx.Process(samples, abortCb, nil, whisper.ProgressCallback(progressCb)); err != nil {
		if p.cancelled.Load() {
			return fmt.Errorf("cancelled")
		}
		return fmt.Errorf("processing audio: %w", err)
	}

	transcribeElapsed := time.Since(processStart).Seconds()
	p.appendLog(fmt.Sprintf("  Transcribed (%.0fs)", transcribeElapsed))
	p.debugLog(fmt.Sprintf("RTF=%.2f (%.1fs audio / %.1fs processing)", audioDurSec/transcribeElapsed, audioDurSec, transcribeElapsed))

	if detected := ctx.DetectedLanguage(); detected != "" {
		p.appendLog(fmt.Sprintf("  Language: %s", detected))
	}

	rawSegs := transcriber.ExtractSegments(ctx)
	segs := transcriber.DeduplicateSegments(rawSegs)
	if dropped := len(rawSegs) - len(segs); dropped > 0 {
		p.debugLog(fmt.Sprintf("dedup: removed %d repeated segments", dropped))
	}

	if !diarOK && usedTDRZ {
		diarSegs = diarizer.SpeakerTurnSegments(segs)
		diarOK = len(diarSegs) > 0
	}

	p.debugLog(fmt.Sprintf("transcript segments=%d diarize segments=%d diarOK=%v", len(segs), len(diarSegs), diarOK))

	detected := ctx.DetectedLanguage()
	if detected == "" {
		detected = language
	}

	diarName := ""
	if diarOK {
		diarName = "nemo-sortformer"
		if usedTDRZ {
			diarName = "tinydiarize"
		}
	}

	device := "cpu"
	for _, dev := range whisper.BackendDevices() {
		if dev.Type == "GPU" || dev.Type == "iGPU" {
			device = dev.Description
			break
		}
	}

	audioDurMs := int64(audioDurSec * 1000)
	transcript := transcriber.BuildTranscript(segs, diarSegs, diarOK, transcriber.TranscriptMeta{
		Model:      modelSize,
		Language:   detected,
		DurationMs: audioDurMs,
		Diarizer:   diarName,
		Device:     device,
	})

	data, err := json.MarshalIndent(transcript, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling transcript: %w", err)
	}
	data = append(data, '\n')

	if cacheKey == "" {
		cacheKey = computeCacheKey(cacheKeyParams{
			SourcePath: absPath,
			MtimeNs:    time.Now().UnixNano(),
			Model:      modelSize,
			Language:   detected,
			Speakers:   speakers,
			NoDiarize:  noDiarize,
		})
	}

	entry := cacheEntry{
		Key:        cacheKey,
		SourcePath: absPath,
		SourceName: sourceName,
		Model:      modelSize,
		Language:   detected,
		Speakers:   speakers,
		NoDiarize:  noDiarize,
		Utterances: len(transcript.Utterances),
		CreatedAt:  time.Now(),
	}
	storedPath, storeErr := cacheStore(entry, data)
	if storeErr != nil {
		return fmt.Errorf("storing transcript: %w", storeErr)
	}

	p.lastCSVPath = storedPath
	p.results = append(p.results, exportItem{cachePath: storedPath, sourceName: sourceName})
	p.appendLog(fmt.Sprintf("  Transcript ready (%d segments)", len(transcript.Utterances)))

	if p.history != nil {
		p.history.refresh()
	}
	return nil
}
