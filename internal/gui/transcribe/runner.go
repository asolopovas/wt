package transcribe

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/appinfo"
	"github.com/asolopovas/wt/internal/diarizer"
	"github.com/asolopovas/wt/internal/gui/cache"
	"github.com/asolopovas/wt/internal/gui/platsvc"
	"github.com/asolopovas/wt/internal/gui/sysstats"
	"github.com/asolopovas/wt/internal/progress"
	"github.com/asolopovas/wt/internal/transcriber"
	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

func notify(title, body string) {
	app := fyne.CurrentApp()
	if app == nil {
		return
	}
	app.SendNotification(&fyne.Notification{Title: title, Content: body})
}

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

func (p *Panel) onTranscribe() {
	p.mu.Lock()
	running := p.running
	p.mu.Unlock()
	if running {
		p.OnCancel()
		return
	}

	if len(p.files) == 0 {
		showNotice(p.window, notifyInfo, "No Files", "Drop audio files to transcribe.")
		return
	}

	if len(p.files) == 1 {
		p.StartTranscription(append([]string(nil), p.files...))
		return
	}

	p.chooseFilesAndTranscribe()
}

func (p *Panel) StartTranscription(files []string) {
	if len(files) == 0 {
		return
	}
	p.onClearLog()
	p.cancelled.Store(false)
	go p.runTranscription(files)
}

func (p *Panel) chooseFilesAndTranscribe() {
	options := make([]string, len(p.files))
	for i, f := range p.files {
		options[i] = filepath.Base(f)
	}
	group := widget.NewCheckGroup(options, nil)
	group.Selected = append([]string(nil), options...)

	scroll := container.NewVScroll(group)
	scroll.SetMinSize(fyne.NewSize(280, 320))

	confirm := func() {
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
		p.StartTranscription(files)
	}

	dialogSize := libraryDialogSize(p.window)
	showDialog(dialogConfig{
		Parent: p.window,
		Title:  "CHOOSE FILES TO TRANSCRIBE",
		Body:   scroll,
		Actions: []dialogAction{
			{Label: "CANCEL", Kind: kindSecondary},
			{Label: "TRANSCRIBE", Kind: kindPrimary, OnTap: confirm},
		},
		Size: &dialogSize,
	})
}

func (p *Panel) runTranscription(files []string) {
	p.setRunning(true)
	defer p.setRunning(false)

	p.results = nil
	p.resetSpeakerRenames()

	notify(appinfo.Name,fmt.Sprintf("Transcribing %d file(s)…", len(files)))

	modelSize := p.Settings.ModelSize()
	device := p.Settings.Device()
	threads := p.Settings.Threads()
	language := p.Settings.Language()
	speakers := p.Settings.Speakers()
	noDiarize := p.Settings.NoDiarize()

	p.AppendLog(fmt.Sprintf("Loading model: %s (%s)...", modelSize, device))
	p.setStatus("Loading model...")
	p.debugLog(fmt.Sprintf("threads=%d language=%q speakers=%d noDiarize=%v", threads, language, speakers, noDiarize))

	model, err := p.loadModel(modelSize)
	if err != nil {
		p.AppendLog(fmt.Sprintf("Error: %v", err))
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
			suffix := ""
			if runtime.GOOS == "android" {
				suffix = ", shared memory"
			}
			p.AppendLog(fmt.Sprintf("Model loaded (%s, %s%s)", modelSize, dev.Description, suffix))
			if runtime.GOOS != "android" {
				usedMB := dev.TotalMB - dev.FreeMB
				p.AppendLog(fmt.Sprintf("VRAM: %d/%d MB", usedMB, dev.TotalMB))
			}
			p.debugLog(fmt.Sprintf("GPU: %s (free=%dMB total=%dMB)", dev.Description, dev.FreeMB, dev.TotalMB))
		}
	}
	if !gpuFound {
		p.AppendLog(fmt.Sprintf("Model loaded (%s, CPU)", modelSize))
		p.debugLog("no GPU detected, using CPU")
	}
	if used, total := sysstats.MemUsageMB(); total > 0 {
		p.AppendLog(fmt.Sprintf("RAM: %d/%d MB", used, total))
	}
	procSnap := sysstats.ProcStats()
	p.AppendLog(fmt.Sprintf("Process: pid=%d threads=%d rss=%dMB cpuset=%s cores-allowed=%d",
		procSnap.PID, procSnap.Threads, procSnap.RSSMB, procSnap.Cpuset, procSnap.NumCores))
	p.debugLog(fmt.Sprintf("system: %d cores total, %d allowed, %s", runtime.NumCPU(), procSnap.NumCores, runtime.GOARCH))

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
			p.AppendLog("Cancelled by user.")
			p.setStatus("Cancelled.")
			notify(appinfo.Name,"Cancelled.")
			return
		}

		p.progBase = float64(i) / float64(total)
		p.progSlice = 1.0 / float64(total)

		filename := filepath.Base(path)
		p.setStatus(fmt.Sprintf("[%d/%d] %s", i+1, total, filename))
		p.setLocalProgress(0)
		p.AppendLog(fmt.Sprintf("[%d/%d] %s", i+1, total, filename))
		p.setChipProcessing(filename, true)
		p.setActivePath(path)

		err := p.transcribeFile(model, path, modelSize, deviceLabel, language, threads, speakers, noDiarize)
		p.setChipProcessing(filename, false)
		p.setActivePath("")
		if err != nil {
			if p.cancelled.Load() {
				p.AppendLog("Cancelled by user.")
				p.setStatus("Cancelled.")
				notify(appinfo.Name,"Cancelled.")
				return
			}
			p.AppendLog(fmt.Sprintf("  Error: %v", err))
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
	p.AppendLog(summary)
	p.setStatus(summary)
	notify(appinfo.Name,summary)
}

func (p *Panel) loadModel(modelSize string) (whisper.Model, error) {
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

	p.AppendLog(fmt.Sprintf("  Model loaded in %.1fs", time.Since(start).Seconds()))
	return m, nil
}

func (p *Panel) transcribeFile(model whisper.Model, path, modelSize, deviceLabel, language string, threads, speakers int, noDiarize bool) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("file not found: %s", absPath)
	}

	sourceName := filepath.Base(absPath)

	params, keyErr := cache.BuildKeyParams(absPath, modelSize, language, speakers, noDiarize)
	var cacheKey string
	if keyErr == nil {
		cacheKey = cache.ComputeKey(params)
		if hitPath, _, ok := cache.Lookup(cacheKey); ok {
			p.lastCSVPath = hitPath
			p.results = append(p.results, ExportItem{CachePath: hitPath, SourceName: sourceName, SourcePath: absPath, CacheKey: cacheKey})
			p.AppendLog(fmt.Sprintf("  Cached transcript reused for %s", sourceName))
			p.setLocalProgress(1.0)
			if p.History != nil {
				p.History.Refresh()
			}
			return nil
		}
	}

	p.setStatus("Loading audio...")
	p.AppendLog("  Loading audio...")
	loadStart := time.Now()
	samples, err := transcriber.LoadAudioSamples(absPath)
	if err != nil {
		return fmt.Errorf("loading audio: %w", err)
	}

	audioDurSec := float64(len(samples)) / transcriber.WhisperSampleRate
	durStr := transcriber.FormatHMS(time.Duration(audioDurSec * float64(time.Second)))
	p.setLocalProgress(0.10)
	p.AppendLog(fmt.Sprintf("  Audio loaded (%s, %.1fs)", durStr, time.Since(loadStart).Seconds()))
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
		rawKey = cache.ComputeRawKey(params.SourcePath, params.MtimeNs, modelSize, language)
		if cached, ok := cache.LoadRawSegments(rawKey); ok {
			segs = cached
			rawHit = true
			detected = language
			p.AppendLog(fmt.Sprintf("  Raw transcript reused (%d segments)", len(cached)))
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
			p.AppendLog("  VAD: Silero v6.2.0")
		}
		transcriber.SetLanguage(ctx, language)

		p.AppendLog("  Transcribing...")
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
				status := fmt.Sprintf("Transcribing... %.1f%%  ETA: %s", disp, formatETA(etaSec))
				p.setStatus(status)
				platsvc.UpdateProgress(int(disp+0.5), fmt.Sprintf("%.1f%% • ETA %s", disp, formatETA(etaSec)))
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
		stopMon := make(chan struct{})
		monDone := make(chan struct{})
		go func() {
			defer close(monDone)
			t := time.NewTicker(5 * time.Second)
			defer t.Stop()
			for {
				select {
				case <-stopMon:
					return
				case <-t.C:
					s := sysstats.ProcStats()
					p.debugLog(fmt.Sprintf("proc: cpu=%d%% threads=%d rss=%dMB cpuset=%s cores=%d",
						s.CPUPct, s.Threads, s.RSSMB, s.Cpuset, s.NumCores))
				}
			}
		}()
		// Pin the process to the lower CPU cluster so the prime/big cores
		// stay free for the UI thread. Worker threads spawned by whisper.cpp
		// inherit affinity from the calling OS thread on Linux.
		runtime.LockOSThread()
		saved, reserved := sysstats.ReserveTopCores(2)
		err = ctx.Process(samples, abortCb, nil, whisper.ProgressCallback(progressCb))
		if reserved {
			sysstats.RestoreAffinity(saved)
		}
		runtime.UnlockOSThread()
		close(stopTick)
		<-tickDone
		close(stopMon)
		<-monDone
		if err != nil {
			if p.cancelled.Load() {
				return fmt.Errorf("cancelled")
			}
			return fmt.Errorf("processing audio: %w", err)
		}
		transcribeElapsed := time.Since(processStart).Seconds()
		p.AppendLog(fmt.Sprintf("  Transcribed (%.0fs)", transcribeElapsed))
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
			p.AppendLog(fmt.Sprintf("  Language: %s", detected))
		} else {
			detected = language
		}
		rawSegs := transcriber.ExtractSegments(ctx)
		segs = transcriber.DeduplicateSegments(rawSegs)
		if dropped := len(rawSegs) - len(segs); dropped > 0 {
			p.debugLog(fmt.Sprintf("dedup: removed %d repeated segments", dropped))
		}
		if rawKey != "" {
			if ok, reason := cache.RawCacheSafe(segs, audioDurSec, p.cancelled.Load()); ok {
				if err := cache.SaveRawSegments(rawKey, segs); err != nil {
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
		cacheKey = cache.ComputeKey(cache.KeyParams{
			SourcePath: absPath,
			MtimeNs:    time.Now().UnixNano(),
			Model:      modelSize,
			Language:   detected,
			Speakers:   speakers,
			NoDiarize:  noDiarize,
		})
	}

	entry := cache.Entry{
		Key:        cacheKey,
		SourcePath: absPath,
		SourceName: sourceName,
		Model:      modelSize,
		Language:   detected,
		Speakers:   speakers,
		NoDiarize:  noDiarize,
		Utterances: len(transcript.Utterances),
		DurationMs: audioDurMs,
		CreatedAt:  time.Now(),
	}
	storedPath, storeErr := cache.Store(entry, data)
	if storeErr != nil {
		return fmt.Errorf("storing transcript: %w", storeErr)
	}

	p.lastCSVPath = storedPath
	p.results = append(p.results, ExportItem{CachePath: storedPath, SourceName: sourceName, SourcePath: absPath, CacheKey: entry.Key})
	p.setLocalProgress(1.0)
	p.AppendLog(fmt.Sprintf("  Transcript ready (%d segments)", len(transcript.Utterances)))

	if p.History != nil {
		p.History.Refresh()
	}
	return nil
}
