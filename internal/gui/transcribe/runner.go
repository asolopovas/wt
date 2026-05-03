package transcribe

import (
	"context"
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
	"github.com/asolopovas/wt/internal/gui/platsvc"
	"github.com/asolopovas/wt/internal/gui/sysstats"
	"github.com/asolopovas/wt/internal/models"
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
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func formatProgressTime(elapsedSec, etaSec float64) string {
	return fmt.Sprintf("%s/%s", formatETA(elapsedSec), formatETA(etaSec))
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

	notify(appinfo.Name, fmt.Sprintf("Transcribing %d file(s)…", len(files)))

	runStart := time.Now()
	shared.LogProcessStart(fmt.Sprintf("transcription (%d file(s))", len(files)))
	p.debugLog(fmt.Sprintf("run start: files=%d", len(files)))

	modelSize := p.Settings.ModelSize()
	device := p.Settings.Device()
	threads := p.Settings.Threads()
	language := p.Settings.Language()
	speakers := p.Settings.Speakers()
	noDiarize := p.Settings.NoDiarize()

	activeEngine := shared.EngineWhisper
	if mgr := models.NewManager(); mgr != nil {
		if eng, _ := models.EngineForActiveASR(mgr.Active(models.FamilyASR)); eng != "" {
			activeEngine = eng
		}
	}

	var model whisper.Model
	if activeEngine == shared.EngineWhisper {
		p.setStatus("Loading model...")
		p.debugLog(fmt.Sprintf("model=%s device=%s threads=%d language=%q speakers=%d noDiarize=%v", modelSize, device, threads, language, speakers, noDiarize))
		var err error
		model, err = p.loadModel(modelSize)
		if err != nil {
			p.AppendLog(fmt.Sprintf("Error: %v", err))
			p.setStatus("Model loading failed")
			p.debugLog(fmt.Sprintf("run done: outcome=failed phase=load-model elapsed=%.1fs reason=%v", time.Since(runStart).Seconds(), err))
			return
		}
		defer func() {
			_ = model.Close()
		}()
	} else {
		p.debugLog(fmt.Sprintf("engine=%s threads=%d language=%q speakers=%d noDiarize=%v (skip whisper load)", activeEngine, threads, language, speakers, noDiarize))
	}

	deviceLabelLog := "CPU"
	gpuFound := false
	if activeEngine == shared.EngineWhisper {
		devices := whisper.BackendDevices()
		for _, dev := range devices {
			if dev.Type == "GPU" || dev.Type == "iGPU" {
				gpuFound = true
				suffix := ""
				if runtime.GOOS == "android" {
					suffix = ", shared mem"
				}
				deviceLabelLog = dev.Description + suffix
				if runtime.GOOS != "android" {
					usedMB := dev.TotalMB - dev.FreeMB
					p.debugLog(fmt.Sprintf("VRAM: %d/%d MB", usedMB, dev.TotalMB))
				}
				p.debugLog(fmt.Sprintf("GPU: %s (free=%dMB total=%dMB)", dev.Description, dev.FreeMB, dev.TotalMB))
			}
		}
		if !gpuFound {
			p.debugLog("no GPU detected, using CPU")
		}
	}

	if activeEngine == shared.EngineWhisper {
		p.AppendLog(fmt.Sprintf("Model: %s · %s", modelSize, deviceLabelLog))
	} else {
		p.AppendLog(fmt.Sprintf("Engine: %s (sherpa-onnx) · %s", activeEngine, deviceLabelLog))
	}

	p.debugLog("Settings:")
	p.debugLog(fmt.Sprintf("  Engine    : %s", activeEngine))
	switch activeEngine {
	case shared.EngineWhisper:
		modelFile := transcriber.ModelFiles[modelSize]
		if modelFile == "" {
			modelFile = modelSize
		}
		if resolved, err := transcriber.ResolveModelPathLocal(modelSize, ""); err == nil {
			p.debugLog(fmt.Sprintf("  Model     : %s (%s)", modelSize, resolved))
		} else {
			p.debugLog(fmt.Sprintf("  Model     : %s (%s)", modelSize, modelFile))
		}
	default:
		p.debugLog(fmt.Sprintf("  ASR model : %s (sherpa-onnx)", modelSize))
	}
	p.debugLog(fmt.Sprintf("  Device    : %s", deviceLabelLog))
	p.debugLog(fmt.Sprintf("  Language  : %s", language))
	p.debugLog(fmt.Sprintf("  Threads   : %d", threads))
	if noDiarize {
		p.debugLog("  Diarizer  : off")
	} else if !diarizer.SupportsExternalBackend() {
		p.debugLog("  Diarizer  : unavailable on this platform")
	} else {
		p.debugLog(fmt.Sprintf("  Speakers  : %d (0 = auto)", speakers))
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
			notify(appinfo.Name, "Cancelled.")
			p.debugLog(fmt.Sprintf("run done: outcome=cancelled phase=between-files done=%d/%d failed=%d elapsed=%.1fs", i, total, errCount, time.Since(runStart).Seconds()))
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

		fileStart := time.Now()
		err := p.transcribeFile(model, path, modelSize, deviceLabel, language, threads, speakers, noDiarize)
		p.setChipProcessing(filename, false)
		p.setActivePath("")
		if err != nil {
			if p.cancelled.Load() {
				p.AppendLog("Cancelled by user.")
				p.setStatus("Cancelled.")
				notify(appinfo.Name, "Cancelled.")
				p.debugLog(fmt.Sprintf("file done: file=%q outcome=cancelled reason=%v elapsed=%.1fs", filename, err, time.Since(fileStart).Seconds()))
				p.debugLog(fmt.Sprintf("run done: outcome=cancelled phase=in-file done=%d/%d failed=%d elapsed=%.1fs", i, total, errCount, time.Since(runStart).Seconds()))
				return
			}
			p.AppendLog(fmt.Sprintf("  Error: %v", err))
			p.debugLog(fmt.Sprintf("file done: file=%q outcome=failed reason=%v elapsed=%.1fs", filename, err, time.Since(fileStart).Seconds()))
			errCount++
			continue
		}
		p.debugLog(fmt.Sprintf("file done: file=%q outcome=ok elapsed=%.1fs", filename, time.Since(fileStart).Seconds()))
	}

	p.setProgress(1.0)

	elapsed := time.Since(runStart).Seconds()
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
	notify(appinfo.Name, summary)
	outcome := "ok"
	if errCount > 0 {
		outcome = "partial"
	}
	p.debugLog(fmt.Sprintf("run done: outcome=%s done=%d/%d failed=%d elapsed=%.1fs", outcome, total-errCount, total, errCount, elapsed))
	shared.LogProcessEnd("transcription", outcome,
		fmt.Sprintf("%d/%d done, %d failed, %.1fs", total-errCount, total, errCount, elapsed))
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
	runtime.LockOSThread()
	prio, lowered := sysstats.SetCurrentThreadBackground()
	m, err := whisper.New(path)
	if lowered {
		sysstats.RestoreThreadPriority(prio)
	}
	runtime.UnlockOSThread()
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
	if st, statErr := os.Stat(absPath); statErr == nil {
		p.debugLog(fmt.Sprintf("transcribeFile: path=%q size=%dB", absPath, st.Size()))
	} else {
		p.debugLog(fmt.Sprintf("transcribeFile: path=%q size=unknown (%v)", absPath, statErr))
	}

	jobCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.mu.Lock()
	p.cancelFunc = cancel
	p.mu.Unlock()
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-jobCtx.Done():
				return
			case <-t.C:
				if p.cancelled.Load() {
					cancel()
					return
				}
			}
		}
	}()
	defer func() { cancel(); <-watchDone }()

	dia := p.resolveDiarizer(speakers, noDiarize)

	audioDurMs := transcriber.ProbeDurationMs(absPath)
	audioDurSec := float64(audioDurMs) / 1000.0
	initialRTF := loadRTF(modelSize, deviceLabel)
	remainSec := audioDurSec
	if remainSec <= 0 {
		remainSec = 1
	}
	smoother := progress.NewSmoother(remainSec, initialRTF)
	var (
		startFrac    float64
		lastTickStop chan struct{}
		tickDone     chan struct{}
	)
	startTickerOnce := func() {
		if lastTickStop != nil {
			return
		}
		lastTickStop = make(chan struct{})
		tickDone = make(chan struct{})
		stop := lastTickStop
		done := tickDone
		go func() {
			defer close(done)
			t := time.NewTicker(500 * time.Millisecond)
			defer t.Stop()
			render := func() {
				rawDisp, etaSec := smoother.Snapshot()
				disp := startFrac*100 + rawDisp*(1-startFrac)
				if disp > 99.5 {
					disp = 99.5
				}
				p.setLocalProgress(0.10 + disp/100.0*0.70)
				elapsed := smoother.Elapsed().Seconds()
				timeStr := formatProgressTime(elapsed, etaSec)
				status := fmt.Sprintf("%.1f%% · %s", disp, timeStr)
				p.setStatus(status)
				platsvc.UpdateProgress(int(disp+0.5), fmt.Sprintf("%.1f%% · %s", disp, timeStr))
			}
			render()
			for {
				select {
				case <-stop:
					return
				case <-t.C:
					render()
				}
			}
		}()
	}
	stopTicker := func() {
		if lastTickStop != nil {
			close(lastTickStop)
			<-tickDone
			lastTickStop = nil
			tickDone = nil
		}
	}
	defer stopTicker()

	phaseStart := map[transcriber.Phase]time.Time{}
	phaseLabels := map[transcriber.Phase]string{
		transcriber.PhaseCacheCheck:   "cache check",
		transcriber.PhaseLoadingAudio: "loading audio",
		transcriber.PhaseTranscribing: "transcribing",
		transcriber.PhaseDiarizing:    "diarizing",
		transcriber.PhaseWriting:      "writing transcript",
	}
	hooks := transcriber.Hooks{
		OnPhase: func(phase transcriber.Phase) {
			phaseStart[phase] = time.Now()
			if label, ok := phaseLabels[phase]; ok {
				p.debugLog("phase: " + label)
			}
			switch phase {
			case transcriber.PhaseCacheCheck:
				p.setStatus("Checking cache...")
			case transcriber.PhaseLoadingAudio:
				p.setStatus("Loading audio...")
				p.setLocalProgress(0.05)
			case transcriber.PhaseTranscribing:
				p.setLocalProgress(0.10)
				startTickerOnce()
			case transcriber.PhaseDiarizing:
				stopTicker()
				p.setLocalProgress(0.80)
				p.setStatus("Diarizing...")
			case transcriber.PhaseWriting:
				stopTicker()
				p.setStatus("Writing transcript...")
				p.setLocalProgress(0.97)
			}
		},
		OnProgress: func(pr transcriber.Progress) {
			if pr.Phase == transcriber.PhaseTranscribing {
				smoother.Report(int(pr.Pct + 0.5))
				return
			}
			if pr.Phase == transcriber.PhaseDiarizing {
				p.setStatus(fmt.Sprintf("Diarizing... %.0f%%", pr.Pct))
				p.setLocalProgress(0.80 + pr.Pct/100.0*0.17)
			}
		},
		OnLog: func(level, msg string) {
			if level == "debug" {
				p.debugLog(msg)
				return
			}
			p.AppendLog("  " + msg)
		},
		OnResume: func(rp transcriber.ResumePrompt) transcriber.ResumeChoice {
			switch p.promptResume(rp.SourceName, rp.ResumeAt, rp.Segments) {
			case resumeYes:
				if audioDurSec > 0 {
					startFrac = rp.ResumeAt.Seconds() / audioDurSec
					if startFrac < 0 {
						startFrac = 0
					}
					if startFrac > 0.999 {
						startFrac = 0.999
					}
					rem := audioDurSec - rp.ResumeAt.Seconds()
					if rem > 0 {
						smoother = progress.NewSmoother(rem, initialRTF)
					}
				}
				return transcriber.ResumeYes
			case resumeAbort:
				p.cancelled.Store(true)
				return transcriber.ResumeAbort
			default:
				return transcriber.ResumeFresh
			}
		},
	}

	engine := shared.EngineWhisper
	if mgr := models.NewManager(); mgr != nil {
		if eng, _ := models.EngineForActiveASR(mgr.Active(models.FamilyASR)); eng != "" {
			engine = eng
		}
	}

	job := &transcriber.Job{Model: model, Diarizer: dia, Hooks: hooks}
	spec := transcriber.JobSpec{
		SourcePath:  absPath,
		ModelSize:   modelSize,
		Language:    language,
		Engine:      engine,
		Threads:     threads,
		Speakers:    speakers,
		NoDiarize:   noDiarize,
		DeviceLabel: deviceLabel,
	}

	runtime.LockOSThread()
	saved, reserved := sysstats.ReserveTopCores(2)
	prio, lowered := sysstats.SetCurrentThreadBackground()
	pinStop := make(chan struct{})
	pinDone := make(chan struct{})
	go func() {
		defer close(pinDone)
		sysstats.PinNewThreadsBackground(pinStop, syscallGettid())
	}()
	res, runErr := job.Run(jobCtx, spec)
	close(pinStop)
	<-pinDone
	if lowered {
		sysstats.RestoreThreadPriority(prio)
	}
	if reserved {
		sysstats.RestoreAffinity(saved)
	}
	runtime.UnlockOSThread()
	stopTicker()

	if runErr != nil {
		if p.cancelled.Load() || jobCtx.Err() != nil {
			return fmt.Errorf("cancelled")
		}
		return runErr
	}

	if res.RTF > 0 {
		saveRTF(modelSize, deviceLabel, res.RTF)
	}
	p.setLocalProgress(1.0)

	p.lastCSVPath = res.CachePath
	spkSet := map[int]struct{}{}
	for _, u := range res.Transcript.Utterances {
		_ = u
	}
	parts := []string{fmt.Sprintf("%.0fs", time.Since(fileStart).Seconds()), fmt.Sprintf("%d segs", len(res.Transcript.Utterances))}
	if res.DetectedLanguage != "" {
		parts = append(parts, res.DetectedLanguage)
	}
	if res.DiarizerName != "" && res.Transcript.SpeakersDetected > 0 {
		parts = append(parts, fmt.Sprintf("%d spk", res.Transcript.SpeakersDetected))
	}
	p.AppendLog("  ✓ " + strings.Join(parts, " · "))
	_ = spkSet

	newSrc, newName := p.autoRenameAfterTranscribe(res.CacheKey, res.CachePath, absPath, sourceName, time.Time{})
	p.results = append(p.results, ExportItem{CachePath: res.CachePath, SourceName: newName, SourcePath: newSrc, CacheKey: res.CacheKey})

	if p.History != nil {
		p.History.Refresh()
	}
	return nil
}

func (p *Panel) resolveDiarizer(speakers int, noDiarize bool) diarizer.Backend {
	if noDiarize || !diarizer.SupportsExternalBackend() {
		return nil
	}
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
		return nil
	}
	dia, err := diarizer.NewWithPreference(speakers, speakers > 0)
	if err != nil {
		p.AppendLog(fmt.Sprintf("  Diarization unavailable: %v", err))
		return nil
	}
	p.debugLog(fmt.Sprintf("diarizer=%s speakers=%d", dia.Name(), speakers))
	return dia
}
