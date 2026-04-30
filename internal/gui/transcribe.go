//go:build !android

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	audioExtensionList = baseAudioExtensions
	audioExtensions    = extensionSet(audioExtensionList)
)

func (p *transcribePanel) build() {
	chipsFlow := newFlowLayout(8)
	p.fileChips = container.New(chipsFlow)
	chipsFlow.setParent(p.fileChips)

	outerBg := canvas.NewRectangle(colSurfLowest)
	outerBg.StrokeColor = colGhostBorder
	outerBg.StrokeWidth = 1

	p.libraryHost = container.NewStack()

	p.dropArea = container.NewStack(outerBg, container.NewPadded(p.libraryHost))

	p.clearBtn = newPointerButton("CLEAR ALL", p.onClear)
	p.clearCacheBtn = newPointerButton("CLEAR CACHE", p.onClearCache)
	p.transcribeBtn = newPointerButton("TRANSCRIBE", p.onTranscribe)
	p.transcribeBtn.Importance = widget.HighImportance

	p.progress = newThinProgress()

	p.statusText = canvas.NewText("READY", colMuted)
	p.statusText.TextSize = 11
	p.statusText.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}

	p.timerText = canvas.NewText("", colMuted)
	p.timerText.TextSize = 11
	p.timerText.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	p.timerText.Alignment = fyne.TextAlignTrailing

	p.statsLine = widget.NewLabel("")
	p.statsLine.TextStyle = fyne.TextStyle{Monospace: true}

	p.logEntry = widget.NewMultiLineEntry()
	p.logEntry.TextStyle = fyne.TextStyle{Monospace: true}
	p.logEntry.Wrapping = fyne.TextWrapWord
	p.logEntry.Disable()

	p.copyLogBtn = newPointerButtonWithIcon("", theme.ContentCopyIcon(), p.onCopyLog)
	p.copyLogBtn.Importance = widget.LowImportance

	p.clearLogBtn = newPointerButtonWithIcon("", theme.HistoryIcon(), p.onClearLog)
	p.clearLogBtn.Importance = widget.LowImportance

	p.autoScroll.Store(true)
	p.autoBtn = newPointerButtonWithIcon("", theme.MoveDownIcon(), nil)
	p.autoBtn.Importance = widget.HighImportance
	p.autoBtn.OnTapped = p.toggleAutoScroll

	appendLogInit(p)

	p.container = container.New(newResponsiveColumns(8), p.dropArea)
}
