package transcribe

import (
	"context"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

const maxLogLines = 5000

type Settings interface {
	ModelSize() string
	Language() string
	Speakers() int
	Threads() int
	NoDiarize() bool
	Device() string
	Debug() bool
}

type History interface {
	Refresh()
	Container() fyne.CanvasObject
}

type Panel struct {
	window   fyne.Window
	Settings Settings
	History  History

	dropArea      *fyne.Container
	dropText      *canvas.Text
	fileChips     *fyne.Container
	LibraryHost   *fyne.Container
	files         []string
	clearBtn      *pointerButton
	clearCacheBtn *pointerButton
	TranscribeBtn *pointerButton

	speakerRenames map[string]string

	Progress   *thinProgress
	StatusText *canvas.Text
	TimerText  *canvas.Text
	StatsLine  *widget.Label

	runStart    time.Time
	timerStop   chan struct{}
	timerStopMu sync.Mutex

	LogEntry     *widget.Entry
	logBufMu     sync.Mutex
	logBuf       []string
	logFlushCh   chan struct{}
	logFlushStop chan struct{}

	autoScroll  atomic.Bool
	AutoBtn     *pointerButton
	CopyLogBtn  *pointerButton
	ClearLogBtn *pointerButton

	lastCSVPath string
	results     []ExportItem

	mu         sync.Mutex
	running    bool
	cancelled  atomic.Bool
	cancelFunc context.CancelFunc
	Container  fyne.CanvasObject

	progressTarget atomic.Uint64
	statusTarget   atomic.Pointer[string]
	smoothStop     chan struct{}
	smoothMu       sync.Mutex

	progBase  float64
	progSlice float64
}

func (p *Panel) setLocalProgress(local float64) {
	if local < 0 {
		local = 0
	}
	if local > 1 {
		local = 1
	}
	slice := p.progSlice
	if slice <= 0 {
		slice = 1
	}
	p.setProgress(p.progBase + local*slice)
}

func (p *Panel) startSmoothUpdates() {
	p.smoothMu.Lock()
	defer p.smoothMu.Unlock()
	if p.smoothStop != nil {
		return
	}
	stop := make(chan struct{})
	p.smoothStop = stop
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		var current float64
		var lastStatus string
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				target := math.Float64frombits(p.progressTarget.Load())
				if target < current {
					current = target
				} else if target > current {
					delta := target - current
					if delta > 0.001 {
						current += delta * 0.5
						if current < 0.02 && target >= 0.02 {
							current = 0.02
						}
					} else {
						current = target
					}
				}
				var statusChanged bool
				var nextStatus string
				if sp := p.statusTarget.Load(); sp != nil && *sp != lastStatus {
					nextStatus = *sp
					lastStatus = nextStatus
					statusChanged = true
				}
				cur := current
				fyne.Do(func() {
					p.Progress.SetValue(cur)
					if statusChanged {
						p.StatusText.Text = nextStatus
						p.StatusText.Refresh()
					}
				})
			}
		}
	}()
}

func (p *Panel) stopSmoothUpdates() {
	p.smoothMu.Lock()
	if p.smoothStop != nil {
		close(p.smoothStop)
		p.smoothStop = nil
	}
	p.smoothMu.Unlock()

	finalProgress := math.Float64frombits(p.progressTarget.Load())
	var finalStatus string
	if sp := p.statusTarget.Load(); sp != nil {
		finalStatus = *sp
	}
	fyne.Do(func() {
		p.Progress.SetValue(finalProgress)
		if finalStatus != "" {
			p.StatusText.Text = finalStatus
			p.StatusText.Refresh()
		}
	})
}

func New(window fyne.Window, settings Settings) *Panel {
	p := &Panel{
		window:     window,
		Settings:   settings,
		logFlushCh: make(chan struct{}, 1),
	}
	p.build()
	p.startLogFlusher()
	p.setupDragDrop()
	p.restorePendingFiles()
	p.startStats()
	return p
}

func (p *Panel) startLogFlusher() {
	p.logFlushStop = make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-p.logFlushStop:
				return
			case <-p.logFlushCh:
				p.flushLogBuffer()
			case <-ticker.C:
				p.flushLogBuffer()
			}
		}
	}()
}

func (p *Panel) flushLogBuffer() {
	p.logBufMu.Lock()
	if len(p.logBuf) == 0 {
		p.logBufMu.Unlock()
		return
	}
	pending := p.logBuf
	p.logBuf = nil
	p.logBufMu.Unlock()

	chunk := strings.Join(pending, "\n")
	autoScroll := p.autoScroll.Load()
	fyne.Do(func() {
		if p.LogEntry == nil {
			return
		}
		text := p.LogEntry.Text
		if text == "" {
			text = chunk
		} else {
			text = text + "\n" + chunk
		}
		if newlines := strings.Count(text, "\n") + 1; newlines > maxLogLines {
			lines := strings.Split(text, "\n")
			lines = lines[len(lines)-maxLogLines:]
			text = strings.Join(lines, "\n")
		}
		p.LogEntry.SetText(text)
		if autoScroll {
			p.LogEntry.CursorRow = strings.Count(text, "\n")
			p.LogEntry.CursorColumn = len(text) - strings.LastIndex(text, "\n") - 1
			p.LogEntry.Refresh()
		}
	})
}

func (p *Panel) setupDragDrop() {
	p.window.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		p.addDroppedFiles(uris)
	})
}
