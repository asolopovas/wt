package transcribe

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/transcriber/cache"
)

var baseAudioExtensions = []string{
	".wav", ".mp3", ".ogg", ".flac", ".m4a", ".m4b", ".wma", ".aac", ".opus",
	".webm", ".mp4", ".mka", ".3gp", ".amr",
}

const startTimeLayout = "2006-01-02 15:04:05"

func (p *Panel) displayName(speaker string) string {
	if alias, ok := p.speakerRenames[speaker]; ok {
		trimmed := strings.TrimSpace(alias)
		if trimmed != "" {
			return trimmed
		}
	}
	return speaker
}

func (p *Panel) resetSpeakerRenames() {
	p.speakerRenames = nil
}

func extensionSet(extensions []string) map[string]bool {
	set := make(map[string]bool, len(extensions))
	for _, ext := range extensions {
		set[ext] = true
	}
	return set
}

func appendLogInit(p *Panel) {
	p.AppendLog("Ready.")
}

func clearCache(window fyne.Window, appendLog func(string)) {
	cacheDir := shared.CacheDir()
	if err := os.RemoveAll(cacheDir); err != nil {
		showError(window, fmt.Errorf("clearing cache: %w", err))
		return
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		showError(window, fmt.Errorf("recreating cache dir: %w", err))
		return
	}
	appendLog("Cache cleared: " + cacheDir)
}

func libraryDialogSize(w fyne.Window) fyne.Size {
	cs := w.Canvas().Size()
	width := cs.Width * 0.9
	height := cs.Height * 0.85
	if width < 360 {
		width = 360
	}
	if height < 480 {
		height = 480
	}
	return fyne.NewSize(width, height)
}

func (p *Panel) addDroppedFiles(uris []fyne.URI) {
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
			if p.AddLocalFile(path) {
				added++
			}
		}
		if added > 0 {
			p.RebuildChips()
			p.UpdateDropLabel()
		}
	})

	if len(rejected) > 0 {
		p.AppendLog("Ignored (unsupported format): " + strings.Join(rejected, ", "))
	}
}

func (p *Panel) AddLocalFile(path string) bool {
	base := filepath.Base(path)
	if p.hasFile(path) {
		p.AppendLog("Already added: " + base)
		p.debugLog("AddLocalFile duplicate path=" + path)
		return false
	}
	p.files = append(p.files, path)
	if _, err := cache.StorePending(path); err != nil {
		p.AppendLog("warn: could not record pending entry: " + err.Error())
	}
	if p.History != nil {
		p.History.Refresh()
	}
	p.AppendLog("Added: " + base)
	if info, err := os.Stat(path); err == nil {
		p.debugLog(fmt.Sprintf("AddLocalFile path=%s size=%d total=%d", path, info.Size(), len(p.files)))
	} else {
		p.debugLog(fmt.Sprintf("AddLocalFile path=%s stat-err=%v total=%d", path, err, len(p.files)))
	}
	return true
}

func (p *Panel) restorePendingFiles() {
	for _, e := range cache.EntriesByRecent() {
		if !e.Pending {
			continue
		}
		if _, err := os.Stat(e.SourcePath); err != nil {
			_ = cache.Delete(e.Key)
			continue
		}
		if p.hasFile(e.SourcePath) {
			continue
		}
		p.files = append(p.files, e.SourcePath)
	}
	p.RebuildChips()
	p.UpdateDropLabel()
}

func (p *Panel) hasFile(path string) bool {
	for _, f := range p.files {
		if f == path {
			return true
		}
	}
	return false
}

func (p *Panel) removeFile(index int) {
	if index < 0 || index >= len(p.files) {
		return
	}
	p.files = append(p.files[:index], p.files[index+1:]...)
	p.RebuildChips()
	p.UpdateDropLabel()
}

func (p *Panel) onClear() {
	p.files = nil
	p.RebuildChips()
	p.UpdateDropLabel()
	p.onClearLog()
	p.setStatus("Ready")
	p.Progress.Hide()
	p.results = nil
	p.resetSpeakerRenames()
}

func (p *Panel) onClearCache() {
	clearCache(p.window, p.AppendLog)
	p.setStatus("Cache cleared.")
}

func (p *Panel) setChipProcessing(filename string, processing bool) {
	fyne.Do(func() {
		if p.fileChips == nil {
			return
		}
		for _, obj := range p.fileChips.Objects {
			chip, ok := obj.(*fileChip)
			if !ok {
				continue
			}
			if chip.name == filename {
				chip.SetProcessing(processing)
			}
		}
	})
}

func (p *Panel) RebuildChips() {
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

func (p *Panel) onClearLog() {
	p.logBufMu.Lock()
	p.logBuf = nil
	p.logLines = nil
	p.logBufMu.Unlock()
	fyne.Do(func() {
		if p.LogEntry != nil {
			p.LogEntry.SetText("")
		}
	})
}

func (p *Panel) onCopyLog() {
	if p.LogEntry == nil {
		return
	}
	fyne.CurrentApp().Clipboard().SetContent(p.LogEntry.Text)
}

func (p *Panel) AppendLog(msg string) {
	line := time.Now().Format("15:04:05") + " " + msg
	p.logBufMu.Lock()
	p.logBuf = append(p.logBuf, line)
	p.logBufMu.Unlock()
	select {
	case p.logFlushCh <- struct{}{}:
	default:
	}

	shared.AppendLogLine(msg)
}

func (p *Panel) setStatus(msg string) {
	p.mu.Lock()
	running := p.running
	p.mu.Unlock()
	upper := strings.ToUpper(msg)
	if running && p.smoothStop != nil {
		s := upper
		p.statusTarget.Store(&s)
		fyne.Do(func() {
			setStatusStyle(p.StatusText, notifyActive)
		})
		return
	}
	level := notifyInfo
	if running {
		level = notifyActive
	}
	fyne.Do(func() {
		setStatusText(p.StatusText, level, msg)
	})
}

func (p *Panel) setProgress(val float64) {
	if p.smoothStop != nil {
		p.progressTarget.Store(math.Float64bits(val))
		return
	}
	fyne.Do(func() {
		p.Progress.SetValue(val)
	})
}

func (p *Panel) makeDownloadProgress(label string) func(downloaded, total int64) {
	var (
		startTime    time.Time
		startOffset  int64
		lastBytes    int64
		lastTime     time.Time
		instRate     float64
		headerLogged bool
	)
	return func(downloaded, total int64) {
		if downloaded < 0 {
			p.AppendLog(fmt.Sprintf("  %s: retry %d — resuming", label, -downloaded))
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
				p.AppendLog(fmt.Sprintf("  %s: resuming %.0f/%.0f MB...",
					label, float64(downloaded)/(1024*1024), float64(total)/(1024*1024)))
			} else {
				p.AppendLog(fmt.Sprintf("  %s: downloading %.0f MB...",
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
		status := fmt.Sprintf("%s %.0f%% • %.0f/%.0fMB • %.1fMB/s • ETA %s",
			label, pct*100, dlMB, totalMB, instRate, eta)
		p.setStatus(status)
		p.setProgress(pct)

		if downloaded == total {
			elapsed := time.Since(startTime).Seconds()
			gotMB := float64(downloaded-startOffset) / (1024 * 1024)
			avg := 0.0
			if elapsed > 0 {
				avg = gotMB / elapsed
			}
			p.AppendLog(fmt.Sprintf("  %s: done — %.0f MB in %.0fs (%.1f MB/s avg)",
				label, gotMB, elapsed, avg))
		}
	}
}
