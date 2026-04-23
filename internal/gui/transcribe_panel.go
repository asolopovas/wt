package gui

import (
	"context"
	"sync"
	"sync/atomic"

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
	timeEntry    *widget.Entry
	startTimeNow *pointerButton

	speakerRenames map[string]string

	progress   *thinProgress
	statusText *canvas.Text
	statsLine  *widget.Label
	logText    *widget.RichText
	logScroll  *container.Scroll

	lastCSVPath string
	results     []exportItem

	mu         sync.Mutex
	running    bool
	cancelled  atomic.Bool
	cancelFunc context.CancelFunc
	container  fyne.CanvasObject
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
