package gui

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	shared "github.com/asolopovas/wt/internal"
	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

var baseAudioExtensions = []string{
	".wav", ".mp3", ".ogg", ".flac", ".m4a", ".m4b", ".wma", ".aac", ".opus",
	".webm", ".mp4", ".mka", ".3gp", ".amr",
}

const (
	startTimeLayout = "2006-01-02 15:04:05"
	timeOnlyLayout  = "15:04:05"
)

func formatTimeOnly(t time.Time) string {
	return t.Format(timeOnlyLayout)
}

func parseTimeOfDay(s string) (hour, min, sec int, err error) {
	t, err := time.Parse(timeOnlyLayout, strings.TrimSpace(s))
	if err != nil {
		return 0, 0, 0, err
	}
	return t.Hour(), t.Minute(), t.Second(), nil
}

func (p *transcribePanel) resolveStartTime() (time.Time, error) {
	if p.dateEntry == nil || p.timeEntry == nil {
		return time.Now(), nil
	}

	date := p.dateEntry.Date
	if date == nil {
		now := time.Now()
		date = &now
	}

	timeText := strings.TrimSpace(p.timeEntry.Text)
	if timeText == "" {
		return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local), nil
	}

	h, m, s, err := parseTimeOfDay(timeText)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time %q (expected HH:MM:SS)", timeText)
	}
	return time.Date(date.Year(), date.Month(), date.Day(), h, m, s, 0, time.Local), nil
}

func (p *transcribePanel) onStartTimeNow() {
	if p.timeEntry == nil {
		return
	}
	current := time.Now()
	if t, err := p.resolveStartTime(); err == nil {
		current = t
	}
	showTimePicker(p.window, current, func(h, m, s int) {
		txt := fmt.Sprintf("%02d:%02d:%02d", h, m, s)
		p.timeEntry.SetText(txt)
		if p.timeBtn != nil {
			p.timeBtn.SetText(txt)
		}
	})
}

func (p *transcribePanel) onStartTimeBothNow() {
	now := time.Now()
	if p.dateEntry != nil {
		p.dateEntry.SetDate(&now)
	}
	if p.dateBtn != nil {
		p.dateBtn.SetText(now.Format("2006-01-02"))
	}
	txt := formatTimeOnly(now)
	if p.timeEntry != nil {
		p.timeEntry.SetText(txt)
	}
	if p.timeBtn != nil {
		p.timeBtn.SetText(txt)
	}
}

func (p *transcribePanel) onPickDate() {
	if p.dateEntry == nil {
		return
	}
	current := time.Now()
	if p.dateEntry.Date != nil {
		current = *p.dateEntry.Date
	}
	showDatePicker(p.window, current, func(t time.Time) {
		p.dateEntry.SetDate(&t)
		if p.dateBtn != nil {
			p.dateBtn.SetText(t.Format("2006-01-02"))
		}
	})
}

func (p *transcribePanel) displayName(speaker string) string {
	if alias, ok := p.speakerRenames[speaker]; ok {
		trimmed := strings.TrimSpace(alias)
		if trimmed != "" {
			return trimmed
		}
	}
	return speaker
}

func (p *transcribePanel) resetSpeakerRenames() {
	p.speakerRenames = nil
}

func extensionSet(extensions []string) map[string]bool {
	set := make(map[string]bool, len(extensions))
	for _, ext := range extensions {
		set[ext] = true
	}
	return set
}

func buildLogPanel(scroll *container.Scroll, leftHeader fyne.CanvasObject, copyBtn, clearLogBtn *pointerButton, extraBtns ...*pointerButton) fyne.CanvasObject {
	bg := canvas.NewRectangle(colSurfLowest)
	bg.StrokeColor = colGhostBorder
	bg.StrokeWidth = 1

	if leftHeader == nil {
		logLabel := canvas.NewText("SYSTEM LOG", colMuted)
		logLabel.TextSize = 10
		logLabel.TextStyle = fyne.TextStyle{Bold: true}
		leftHeader = logLabel
	}

	headerBg := canvas.NewRectangle(colSurfLow)
	headerObjs := []fyne.CanvasObject{leftHeader, layout.NewSpacer()}
	for _, b := range extraBtns {
		headerObjs = append(headerObjs, b)
	}
	headerObjs = append(headerObjs, copyBtn, clearLogBtn)
	headerContent := container.NewHBox(headerObjs...)
	header := container.NewStack(headerBg, container.NewPadded(headerContent))

	inner := container.NewBorder(header, nil, nil, nil, scroll)
	return container.NewStack(bg, inner)
}

func appendLogInit(logText *widget.RichText) {
	entries := []string{
		"Initializing Whisper Core ...",
		"Ready.",
	}
	for _, msg := range entries {
		ts := time.Now().Format("[15:04:05]")
		logText.Segments = append(logText.Segments, &widget.TextSegment{
			Text: ts + "  " + msg,
			Style: widget.RichTextStyle{
				ColorName: theme.ColorNamePrimary,
				TextStyle: fyne.TextStyle{Monospace: true},
			},
		})
	}
}

func clearCache(window fyne.Window, appendLog func(string)) {
	cacheDir := shared.CacheDir()
	if err := os.RemoveAll(cacheDir); err != nil {
		dialog.ShowError(fmt.Errorf("clearing cache: %w", err), window)
		return
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		dialog.ShowError(fmt.Errorf("recreating cache dir: %w", err), window)
		return
	}
	appendLog("Cache cleared: " + cacheDir)
}

func detectDevice() string {
	var parts []string

	whisper.SetLogQuiet(true)
	if exePath, err := os.Executable(); err == nil {
		whisper.BackendSetSearchPath(filepath.Dir(exePath))
	}
	whisper.BackendLoadAll()
	devices := whisper.BackendDevices()
	for _, dev := range devices {
		if dev.Type == "GPU" || dev.Type == "iGPU" {
			info := dev.Description
			if dev.TotalMB > 0 {
				info += fmt.Sprintf(" (%.1f GB)", float64(dev.TotalMB)/1024.0)
			}
			parts = append(parts, info)
		}
	}

	if len(parts) == 0 {
		return "CPU ONLY"
	}
	return strings.Join(parts, " | ")
}

func (p *transcribePanel) addDroppedFiles(uris []fyne.URI) {
	var accepted, rejected []string
	for _, u := range uris {
		path := u.Path()
		if path == "" {
			rejected = append(rejected, u.String())
			continue
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !audioExtensions[ext] {
			rejected = append(rejected, filepath.Base(path))
			continue
		}
		accepted = append(accepted, path)
	}

	fyne.Do(func() {
		added := 0
		for _, path := range accepted {
			if !p.hasFile(path) {
				p.files = append(p.files, path)
				added++
			}
		}
		if added > 0 {
			p.rebuildChips()
			p.updateDropLabel()
		}
	})

	if len(rejected) > 0 {
		p.appendLog("Ignored (unsupported format): " + strings.Join(rejected, ", "))
	}
}

func (p *transcribePanel) hasFile(path string) bool {
	for _, f := range p.files {
		if f == path {
			return true
		}
	}
	return false
}

func (p *transcribePanel) removeFile(index int) {
	if index < 0 || index >= len(p.files) {
		return
	}
	p.files = append(p.files[:index], p.files[index+1:]...)
	p.rebuildChips()
	p.updateDropLabel()
}

func (p *transcribePanel) onClear() {
	p.files = nil
	p.rebuildChips()
	p.updateDropLabel()
	p.logText.Segments = nil
	p.logText.Refresh()
	p.setStatus("Ready")
	p.progress.Hide()
	p.results = nil
	p.resetSpeakerRenames()
}

func (p *transcribePanel) onClearCache() {
	clearCache(p.window, p.appendLog)
	p.setStatus("Cache cleared.")
}

func (p *transcribePanel) rebuildChips() {
	p.fileChips.Objects = nil
	for i, f := range p.files {
		idx := i
		chip := newFileChip(filepath.Base(f), func() {
			p.removeFile(idx)
		})
		p.fileChips.Objects = append(p.fileChips.Objects, chip)
	}
	p.fileChips.Refresh()
}

func (p *transcribePanel) onClearLog() {
	fyne.Do(func() {
		p.logText.Segments = nil
		p.logText.Refresh()
	})
}

func (p *transcribePanel) onCopyLog() {
	var sb strings.Builder
	for _, seg := range p.logText.Segments {
		if ts, ok := seg.(*widget.TextSegment); ok {
			sb.WriteString(ts.Text)
			sb.WriteByte('\n')
		}
	}
	fyne.CurrentApp().Clipboard().SetContent(sb.String())
}

func (p *transcribePanel) appendLog(msg string) {
	fyne.Do(func() {
		ts := time.Now().Format("[15:04:05]")
		seg := &widget.TextSegment{
			Text: ts + "  " + msg,
			Style: widget.RichTextStyle{
				ColorName: theme.ColorNamePrimary,
				TextStyle: fyne.TextStyle{Monospace: true},
			},
		}
		p.logText.Segments = append(p.logText.Segments, seg)
		p.logText.Refresh()
		if p.autoScroll.Load() {
			p.logScroll.ScrollToBottom()
		}
	})
}

func (p *transcribePanel) setStatus(msg string) {
	p.mu.Lock()
	running := p.running
	p.mu.Unlock()
	upper := strings.ToUpper(msg)
	if running && p.smoothStop != nil {
		s := upper
		p.statusTarget.Store(&s)
		fyne.Do(func() {
			p.statusText.Color = colSecondary
			p.statusText.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
		})
		return
	}
	fyne.Do(func() {
		if running {
			p.statusText.Color = colSecondary
			p.statusText.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
		} else {
			p.statusText.Color = colMuted
			p.statusText.TextStyle = fyne.TextStyle{Monospace: true}
		}
		p.statusText.Text = upper
		p.statusText.Refresh()
	})
}

func (p *transcribePanel) setProgress(val float64) {
	if p.smoothStop != nil {
		p.progressTarget.Store(math.Float64bits(val))
		return
	}
	fyne.Do(func() {
		p.progress.SetValue(val)
	})
}

var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

func (p *transcribePanel) makeDownloadProgress(label string) func(downloaded, total int64) {
	var (
		startTime    time.Time
		startOffset  int64
		lastBytes    int64
		lastTime     time.Time
		instRate     float64
		headerLogged bool
		ticks        int
	)
	return func(downloaded, total int64) {
		if downloaded < 0 {
			p.appendLog(fmt.Sprintf("  %s: retry %d — resuming", label, -downloaded))
			lastBytes = 0
			startTime = time.Time{}
			return
		}
		if total <= 0 {
			return
		}
		if startTime.IsZero() {
			startTime = time.Now()
			startOffset = downloaded
			lastBytes = downloaded
			lastTime = startTime
		}
		if !headerLogged {
			if downloaded > 0 && downloaded < total {
				p.appendLog(fmt.Sprintf("  %s: resuming %.0f/%.0f MB...",
					label, float64(downloaded)/(1024*1024), float64(total)/(1024*1024)))
			} else {
				p.appendLog(fmt.Sprintf("  %s: downloading %.0f MB...",
					label, float64(total)/(1024*1024)))
			}
			headerLogged = true
		}

		now := time.Now()
		dt := now.Sub(lastTime).Seconds()
		if dt >= 0.4 {
			delta := downloaded - lastBytes
			if delta > 0 {
				cur := float64(delta) / dt / (1024 * 1024)
				if instRate == 0 {
					instRate = cur
				} else {
					instRate = instRate*0.7 + cur*0.3
				}
			}
			lastBytes = downloaded
			lastTime = now
		}

		pct := float64(downloaded) / float64(total)
		dlMB := float64(downloaded) / (1024 * 1024)
		totalMB := float64(total) / (1024 * 1024)
		eta := "--"
		if instRate > 0 && downloaded < total {
			eta = formatETA((totalMB - dlMB) / instRate)
		}
		ticks++
		spinner := string(spinnerFrames[ticks%len(spinnerFrames)])
		status := fmt.Sprintf("%s %s %.0f%% • %.0f/%.0fMB • %.1fMB/s • ETA %s",
			spinner, label, pct*100, dlMB, totalMB, instRate, eta)
		p.setStatus(status)
		p.setProgress(pct)

		if downloaded == total {
			elapsed := time.Since(startTime).Seconds()
			gotMB := float64(downloaded-startOffset) / (1024 * 1024)
			avg := 0.0
			if elapsed > 0 {
				avg = gotMB / elapsed
			}
			p.appendLog(fmt.Sprintf("  %s: done — %.0f MB in %.0fs (%.1f MB/s avg)",
				label, gotMB, elapsed, avg))
		}
	}
}
