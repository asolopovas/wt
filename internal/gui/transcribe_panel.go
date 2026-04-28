package gui

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type transcribePanel struct {
	window   fyne.Window
	settings *settingsPanel
	history  *historyPanel

	dropArea      *fyne.Container
	dropText      *canvas.Text
	fileChips     *fyne.Container
	files         []string
	clearBtn      *pointerButton
	clearCacheBtn *pointerButton
	transcribeBtn *pointerButton
	previewBtn    *pointerButton
	exportBtn     *pointerButton
	openBtn       *pointerButton

	dateEntry    *widget.DateEntry
	timeEntry    *tappableEntry
	startTimeNow *pointerButton
	dateBtn      *widget.Button
	timeBtn      *widget.Button

	speakerRenames map[string]string

	progress   *thinProgress
	statusText *canvas.Text
	statsLine  *widget.Label
	logText    *widget.RichText
	logScroll  *container.Scroll
	autoScroll atomic.Bool
	autoBtn    *pointerButton

	lastCSVPath string
	results     []exportItem

	mu         sync.Mutex
	running    bool
	cancelled  atomic.Bool
	cancelFunc context.CancelFunc
	container  fyne.CanvasObject

	progressTarget atomic.Uint64
	statusTarget   atomic.Pointer[string]
	smoothStop     chan struct{}
	smoothMu       sync.Mutex
}

func (p *transcribePanel) startSmoothUpdates() {
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
					p.progress.SetValue(cur)
					if statusChanged {
						p.statusText.Text = nextStatus
						p.statusText.Refresh()
					}
				})
			}
		}
	}()
}

func (p *transcribePanel) stopSmoothUpdates() {
	p.smoothMu.Lock()
	defer p.smoothMu.Unlock()
	if p.smoothStop != nil {
		close(p.smoothStop)
		p.smoothStop = nil
	}
}

func newTranscribePanel(window fyne.Window, settings *settingsPanel) *transcribePanel {
	p := &transcribePanel{
		window:   window,
		settings: settings,
	}
	p.build()
	p.setupDragDrop()
	p.startStats()
	return p
}

func (p *transcribePanel) setupDragDrop() {
	p.window.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		p.addDroppedFiles(uris)
	})
}
