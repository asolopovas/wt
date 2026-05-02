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

	p.setStatus("Loading model...")
	p.debugLog(fmt.Sprintf("model=%s device=%s threads=%d language=%q speakers=%d noDiarize=%v", modelSize, device, threads, language, speakers, noDiarize))

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
				suffix = ", shared mem"
			}
			p.AppendLog(fmt.Sprintf("Model: %s · %s%s", modelSize, dev.Description, suffix))
			if runtime.GOOS != "android" {
				usedMB := dev.TotalMB - dev.FreeMB
				p.debugLog(fmt.Sprintf("VRAM: %d/%d MB", usedMB, dev.TotalMB))
			}
			p.debugLog(fmt.Sprintf("GPU: %s (free=%dMB total=%dMB)", dev.Description, dev.FreeMB, dev.TotalMB))
		}
	}
	if !gpuFound {
		p.AppendLog(fmt.Sprintf("Model: %s · CPU", modelSize))
		p.debugLog("no GPU detected, using CPU")
	}
	if used, total := sysstats.MemUsageMB(); total > 0 {
		p.debugLog(fmt.Sprintf("RAM: %d/%d MB", used, total))
	}
	procSnap := sysstats.ProcStats()
	p.debugLog(fmt.Sprintf("Process: pid=%d threads=%d rss=%dMB cpuset=%s cores-allowed=%d",
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

	p.debugLog(fmt.Sprintf("model loaded in %.1fs", time.Since(start).Seconds()))
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
	fileStart := time.Now()

	params, keyErr := cache.BuildKeyParams(absPath, modelSize, language, speakers, noDiarize)
	var cacheKey string
	if keyErr == nil {
		cacheKey = cache.ComputeKey(params)
		if hitPath, _, ok := cache.Lookup(cacheKey); ok {
			p.lastCSVPath = hitPath
			p.results = append(p.results, ExportItem{CachePath: hitPath, SourceName: sourceName, SourcePath: absPath, CacheKey: cacheKey})
			p.AppendLog("  ⚡ cached transcript reused")
			p.setLocalProgress(1.0)
			if p.History != nil {
				p.History.Refresh()
			}
			return nil
		}
	}

	p.setStatus("Loading audio...")
	loadStart := time.Now()
	samples, err := transcriber.LoadAudioSamples(absPath)
	if err != nil {
		return fmt.Errorf("loading audio: %w", err)
	}

	audioDurSec := float64(len(samples)) / transcriber.WhisperSampleRate
	durStr := transcriber.FormatHMS(time.Duration(audioDurSec * float64(time.Second)))
	p.setLocalProgress(0.10)
	p.debugLog(fmt.Sprintf("audio loaded (%s, %.1fs) samples=%d rate=%d", durStr, time.Since(loadStart).Seconds(), len(samples), transcriber.WhisperSampleRate))

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
			p.AppendLog(fmt.Sprintf("  ⚡ raw transcript reused (%d segs)", len(cached)))
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
			p.debugLog("VAD: Silero v6.2.0")
		}
		transcriber.SetLanguage(ctx, language)

		var (
			resumeSegs []diarizer.TranscriptSegment
			offsetMs   int64
		)
		if rawKey != "" {
			if part, ok := cache.LoadPartial(rawKey); ok {
				resumeAt := time.Duration(part.LastEndMs) * time.Millisecond
				switch p.promptResume(sourceName, resumeAt, len(part.Segments)) {
				case resumeYes:
					resumeSegs = part.Segments
					offsetMs = part.LastEndMs
					ctx.SetOffset(time.Duration(offsetMs) * time.Millisecond)
					p.AppendLog(fmt.Sprintf("  Resuming from %s (%d segments cached)",
						transcriber.FormatHMS(resumeAt), len(resumeSegs)))
				case resumeFresh:
					cache.DeletePartial(rawKey)
					p.AppendLog("  Discarded partial transcript; starting from beginning.")
				case resumeAbort:
					p.cancelled.Store(true)
					return fmt.Errorf("cancelled")
				}
			}
		}

		offsetSec := float64(offsetMs) / 1000.0
		startFrac := 0.0
		if audioDurSec > 0 {
			startFrac = offsetSec / audioDurSec
		}
		if startFrac < 0 {
			startFrac = 0
		}
		if startFrac > 0.999 {
			startFrac = 0.999
		}
		remainDurSec := audioDurSec - offsetSec
		if remainDurSec <= 0 {
			remainDurSec = 1
		}

		processStart := time.Now()
		initialRTF := loadRTF(modelSize, deviceLabel)
		smoother := progress.NewSmoother(remainDurSec, initialRTF)
		stopTick := make(chan struct{})
		tickDone := make(chan struct{})
		go func() {
			defer close(tickDone)
			t := time.NewTicker(200 * time.Millisecond)
			defer t.Stop()
			render := func() {
				rawDisp, etaSec := smoother.Snapshot()
				disp := startFrac*100 + rawDisp*(1-startFrac)
				if disp > 99.5 {
					disp = 99.5
				}
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

		newSegs := transcriber.ExtractSegments(ctx)
		merged := make([]diarizer.TranscriptSegment, 0, len(resumeSegs)+len(newSegs))
		merged = append(merged, resumeSegs...)
		merged = append(merged, newSegs...)

		if err != nil {
			if p.cancelled.Load() {
				if rawKey != "" {
					p.savePartialIfUseful(rawKey, merged, audioDurSec)
				}
				return fmt.Errorf("cancelled")
			}
			return fmt.Errorf("processing audio: %w", err)
		}
		transcribeElapsed := time.Since(processStart).Seconds()
		p.debugLog(fmt.Sprintf("transcribed in %.0fs", transcribeElapsed))
		observedRTF := 0.0
		if transcribeElapsed > 0 && remainDurSec > 0 {
			observedRTF = remainDurSec / transcribeElapsed
		}
		p.debugLog(fmt.Sprintf("RTF=%.2f (%.1fs audio / %.1fs processing)", observedRTF, remainDurSec, transcribeElapsed))
		if observedRTF > 0 {
			saveRTF(modelSize, deviceLabel, observedRTF)
		}
		p.setLocalProgress(0.80)

		detected = ctx.DetectedLanguage()
		if detected == "" {
			detected = language
		}
		segs = transcriber.DeduplicateSegments(merged)
		if dropped := len(merged) - len(segs); dropped > 0 {
			p.debugLog(fmt.Sprintf("dedup: removed %d repeated segments", dropped))
		}
		if rawKey != "" {
			if ok, reason := cache.RawCacheSafe(segs, audioDurSec, p.cancelled.Load()); ok {
				if err := cache.SaveRawSegments(rawKey, segs); err != nil {
					p.debugLog(fmt.Sprintf("could not save raw transcript cache: %v", err))
				}
				cache.DeletePartial(rawKey)
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
	p.setLocalProgress(1.0)
	spkSet := make(map[int]struct{})
	for _, s := range diarSegs {
		spkSet[s.Speaker] = struct{}{}
	}
	parts := []string{fmt.Sprintf("%.0fs", time.Since(fileStart).Seconds()), fmt.Sprintf("%d segs", len(transcript.Utterances))}
	if detected != "" {
		parts = append(parts, detected)
	}
	if diarOK && len(spkSet) > 0 {
		parts = append(parts, fmt.Sprintf("%d spk", len(spkSet)))
	}
	p.AppendLog("  ✓ " + strings.Join(parts, " · "))

	newSrc, newName := p.autoRenameAfterTranscribe(entry.Key, storedPath, absPath, sourceName, time.Time{})
	p.results = append(p.results, ExportItem{CachePath: storedPath, SourceName: newName, SourcePath: newSrc, CacheKey: entry.Key})

	if p.History != nil {
		p.History.Refresh()
	}
	return nil
}
