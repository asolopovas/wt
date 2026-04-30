package gui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/diarizer"
	"github.com/asolopovas/wt/internal/progress"
	"github.com/asolopovas/wt/internal/transcriber"
	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

func formatETA(secs float64) string {
	if secs < 0 {
		secs = 0
	}
	total := int(secs)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
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

	if len(p.files) == 1 {
		p.startTranscription(append([]string(nil), p.files...))
		return
	}

	p.chooseFilesAndTranscribe()
}

func (p *transcribePanel) startTranscription(files []string) {
	if len(files) == 0 {
		return
	}
	p.onClearLog()
	p.cancelled.Store(false)
	go p.runTranscription(files)
}

func (p *transcribePanel) chooseFilesAndTranscribe() {
	options := make([]string, len(p.files))
	for i, f := range p.files {
		options[i] = filepath.Base(f)
	}
	group := widget.NewCheckGroup(options, nil)
	group.Selected = append([]string(nil), options...)

	scroll := container.NewVScroll(group)
	scroll.SetMinSize(fyne.NewSize(280, 320))

	var hidePopup func()

	confirm := func() {
		if hidePopup != nil {
			hidePopup()
		}
		selectedSet := make(map[string]bool, len(group.Selected))
		for _, s := range group.Selected {
			selectedSet[s] = true
		}
		files := make([]string, 0, len(p.files))
		for i, f := range p.files {
			if selectedSet[options[i]] {
				files = append(files, f)
			}
		}
		if len(files) == 0 {
			return
		}
		p.startTranscription(files)
	}

	cancelBtn := newPointerButton("CANCEL", func() {
		if hidePopup != nil {
			hidePopup()
		}
	})
	cancelBtn.Importance = widget.LowImportance

	transcribeBtn := newPointerButton("TRANSCRIBE", confirm)
	transcribeBtn.Importance = widget.LowImportance

	buttons := container.NewGridWithColumns(2,
		borderedBtn(cancelBtn, colOutline),
		borderedBtn(transcribeBtn, colOutline),
	)
	bottomGap := canvas.NewRectangle(transparent)
	bottomGap.SetMinSize(fyne.NewSize(0, previewBottomInset()))
	actionRow := container.NewVBox(buttons, bottomGap)

	titleLabel := canvas.NewText("CHOOSE FILES TO TRANSCRIBE", colMuted)
	titleLabel.TextSize = 10
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	topGap := canvas.NewRectangle(transparent)
	topGap.SetMinSize(fyne.NewSize(0, previewTopInset()))
	top := container.NewVBox(topGap, container.NewHBox(titleLabel))

	body := container.NewBorder(top, actionRow, nil, nil, scroll)

	pop := widget.NewModalPopUp(dialogBordered(body), p.window.Canvas())
	pop.Resize(libraryDialogSize(p.window))
	hidePopup = pop.Hide
	pop.Show()
}

func (p *transcribePanel) runTranscription(files []string) {
	p.setRunning(true)
	defer p.setRunning(false)

	p.results = nil
	p.resetSpeakerRenames()

	notify("wt", fmt.Sprintf("Transcribing %d file(s)…", len(files)))

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

	total := len(files)
	errCount := 0

	deviceLabel := "cpu"
	for _, dev := range whisper.BackendDevices() {
		if dev.Type == "GPU" || dev.Type == "iGPU" {
			deviceLabel = dev.Description
			break
		}
	}

	for i, path := range files {
		if p.cancelled.Load() {
			p.appendLog("Cancelled by user.")
			p.setStatus("Cancelled.")
			notify("wt", "Cancelled.")
			return
		}

		p.progBase = float64(i) / float64(total)
		p.progSlice = 1.0 / float64(total)

		filename := filepath.Base(path)
		p.setStatus(fmt.Sprintf("[%d/%d] %s", i+1, total, filename))
		p.setLocalProgress(0)
		p.appendLog(fmt.Sprintf("[%d/%d] %s", i+1, total, filename))

		if err := p.transcribeFile(model, path, modelSize, deviceLabel, language, threads, speakers, noDiarize); err != nil {
			if p.cancelled.Load() {
				p.appendLog("Cancelled by user.")
				p.setStatus("Cancelled.")
				notify("wt", "Cancelled.")
				return
			}
			p.appendLog(fmt.Sprintf("  Error: %v", err))
			errCount++
			continue
		}
	}

	p.setProgress(1.0)

	var summary string
	if errCount > 0 {
		summary = fmt.Sprintf("Done: %d/%d transcribed, %d failed.", total-errCount, total, errCount)
	} else if total == 1 {
		summary = "Transcription complete."
	} else {
		summary = fmt.Sprintf("All %d files transcribed.", total)
	}
	p.appendLog(summary)
	p.setStatus(summary)
	notify("wt", summary)
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

func (p *transcribePanel) transcribeFile(model whisper.Model, path, modelSize, deviceLabel, language string, threads, speakers int, noDiarize bool) error {
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
			p.results = append(p.results, exportItem{cachePath: hitPath, sourceName: sourceName, sourcePath: absPath, cacheKey: cacheKey})
			p.appendLog(fmt.Sprintf("  Cached transcript reused for %s", sourceName))
			p.setLocalProgress(1.0)
			if p.history != nil {
				p.history.refresh()
			}
			return nil
		}
	}

	p.setStatus("Loading audio...")
	p.appendLog("  Loading audio...")
	loadStart := time.Now()
	samples, err := transcriber.LoadAudioSamples(absPath)
	if err != nil {
		return fmt.Errorf("loading audio: %w", err)
	}

	audioDurSec := float64(len(samples)) / transcriber.WhisperSampleRate
	durStr := transcriber.FormatHMS(time.Duration(audioDurSec * float64(time.Second)))
	p.setLocalProgress(0.10)
	p.appendLog(fmt.Sprintf("  Audio loaded (%s, %.1fs)", durStr, time.Since(loadStart).Seconds()))
	p.debugLog(fmt.Sprintf("samples=%d sampleRate=%d duration=%.1fs", len(samples), transcriber.WhisperSampleRate, audioDurSec))

	if p.cancelled.Load() {
		return fmt.Errorf("cancelled")
	}

	var (
		segs     []diarizer.TranscriptSegment
		detected string
		rawKey   string
		rawHit   bool
	)
	if keyErr == nil {
		rawKey = computeRawKey(params.SourcePath, params.MtimeNs, modelSize, language)
		if cached, ok := loadRawSegments(rawKey); ok {
			segs = cached
			rawHit = true
			detected = language
			p.appendLog(fmt.Sprintf("  Raw transcript reused (%d segments)", len(cached)))
			p.setLocalProgress(0.80)
		}
	}

	if !rawHit {
		ctx, err := model.NewContext()
		if err != nil {
			return fmt.Errorf("creating context: %w", err)
		}
		transcriber.ConfigureContext(ctx, transcriber.ContextConfig{
			Threads: threads,
			TDRZ:    false,
		})
		if transcriber.ConfigureVAD(ctx) {
			p.appendLog("  VAD: Silero v6.2.0")
		}
		transcriber.SetLanguage(ctx, language)

		p.appendLog("  Transcribing...")
		processStart := time.Now()
		initialRTF := loadRTF(modelSize, deviceLabel)
		smoother := progress.NewSmoother(audioDurSec, initialRTF)
		stopTick := make(chan struct{})
		tickDone := make(chan struct{})
		go func() {
			defer close(tickDone)
			t := time.NewTicker(200 * time.Millisecond)
			defer t.Stop()
			render := func() {
				disp, etaSec := smoother.Snapshot()
				p.setLocalProgress(0.10 + disp/100.0*0.70)
				p.setStatus(fmt.Sprintf("Transcribing... %.1f%%  ETA: %s", disp, formatETA(etaSec)))
			}
			render()
			for {
				select {
				case <-stopTick:
					return
				case <-t.C:
					render()
				}
			}
		}()
		abortCb := func() bool { return !p.cancelled.Load() }
		progressCb := func(pct int) {
			if pct > 100 {
				pct = 100
			}
			smoother.Report(pct)
		}
		err = ctx.Process(samples, abortCb, nil, whisper.ProgressCallback(progressCb))
		close(stopTick)
		<-tickDone
		if err != nil {
			if p.cancelled.Load() {
				return fmt.Errorf("cancelled")
			}
			return fmt.Errorf("processing audio: %w", err)
		}
		transcribeElapsed := time.Since(processStart).Seconds()
		p.appendLog(fmt.Sprintf("  Transcribed (%.0fs)", transcribeElapsed))
		observedRTF := 0.0
		if transcribeElapsed > 0 {
			observedRTF = audioDurSec / transcribeElapsed
		}
		p.debugLog(fmt.Sprintf("RTF=%.2f (%.1fs audio / %.1fs processing)", observedRTF, audioDurSec, transcribeElapsed))
		if observedRTF > 0 {
			saveRTF(modelSize, deviceLabel, observedRTF)
		}
		p.setLocalProgress(0.80)

		detected = ctx.DetectedLanguage()
		if detected != "" {
			p.appendLog(fmt.Sprintf("  Language: %s", detected))
		} else {
			detected = language
		}
		rawSegs := transcriber.ExtractSegments(ctx)
		segs = transcriber.DeduplicateSegments(rawSegs)
		if dropped := len(rawSegs) - len(segs); dropped > 0 {
			p.debugLog(fmt.Sprintf("dedup: removed %d repeated segments", dropped))
		}
		if rawKey != "" {
			if ok, reason := rawCacheSafe(segs, audioDurSec, p.cancelled.Load()); ok {
				if err := saveRawSegments(rawKey, segs); err != nil {
					p.debugLog(fmt.Sprintf("could not save raw transcript cache: %v", err))
				}
			} else {
				p.debugLog(fmt.Sprintf("skipped raw cache save: %s", reason))
			}
		}
	}

	if p.cancelled.Load() {
		return fmt.Errorf("cancelled")
	}

	var diarSegs []diarizer.Segment
	diarOK := false
	if !noDiarize && diarizer.SupportsExternalBackend() {
		wavPath := transcriber.ResolveWAVPath(absPath)
		if !strings.HasSuffix(strings.ToLower(wavPath), ".wav") || wavPath == absPath {
			audioKey, err := transcriber.AudioCacheKey(absPath)
			if err == nil {
				cachePath := filepath.Join(shared.CacheDir(), audioKey)
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

	p.debugLog(fmt.Sprintf("transcript segments=%d diarize segments=%d diarOK=%v", len(segs), len(diarSegs), diarOK))

	diarName := ""
	if diarOK {
		diarName = "sherpa-onnx-pyannote"
	}

	audioDurMs := int64(audioDurSec * 1000)
	transcript := transcriber.BuildTranscript(segs, diarSegs, diarOK, transcriber.TranscriptMeta{
		Model:      modelSize,
		Language:   detected,
		DurationMs: audioDurMs,
		Diarizer:   diarName,
		Device:     deviceLabel,
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
	p.results = append(p.results, exportItem{cachePath: storedPath, sourceName: sourceName, sourcePath: absPath, cacheKey: entry.Key})
	p.setLocalProgress(1.0)
	p.appendLog(fmt.Sprintf("  Transcript ready (%d segments)", len(transcript.Utterances)))

	if p.history != nil {
		p.history.refresh()
	}
	return nil
}
