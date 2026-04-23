package gui

import (
	"fmt"
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
	if p.dateEntry == nil || p.timeEntry == nil {
		return
	}
	now := time.Now()
	p.dateEntry.SetDate(&now)
	p.timeEntry.SetText(formatTimeOnly(now))
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

func buildLogPanel(scroll *container.Scroll, copyBtn, clearLogBtn *pointerButton) fyne.CanvasObject {
	bg := canvas.NewRectangle(colSurfLowest)
	bg.StrokeColor = colGhostBorder
	bg.StrokeWidth = 1

	logLabel := canvas.NewText("SYSTEM LOG", colMuted)
	logLabel.TextSize = 10
	logLabel.TextStyle = fyne.TextStyle{Bold: true}

	headerBg := canvas.NewRectangle(colSurfLow)
	headerContent := container.NewHBox(logLabel, layout.NewSpacer(), copyBtn, clearLogBtn)
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
	p.updatePreviewAvailability()
}

func (p *transcribePanel) updatePreviewAvailability() {
	if p.previewBtn == nil {
		return
	}
	fyne.Do(func() {
		if len(p.results) == 0 {
			p.previewBtn.Disable()
		} else {
			p.previewBtn.Enable()
		}
	})
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
		p.logScroll.ScrollToBottom()
	})
}

func (p *transcribePanel) setStatus(msg string) {
	p.mu.Lock()
	running := p.running
	p.mu.Unlock()
	fyne.Do(func() {
		upper := strings.ToUpper(msg)
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
	fyne.Do(func() {
		p.progress.SetValue(val)
	})
}
