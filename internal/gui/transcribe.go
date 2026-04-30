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
	chipsFlow := newFlowLayout(spaceLG)
	p.fileChips = container.New(chipsFlow)
	chipsFlow.setParent(p.fileChips)

	p.libraryHost = container.NewStack()

	p.dropArea = container.NewStack(newPanelBackground(), container.NewPadded(p.libraryHost))

	p.clearBtn = newPointerButton("CLEAR ALL", p.onClear)
	p.clearCacheBtn = newPointerButton("CLEAR CACHE", p.onClearCache)
	p.transcribeBtn = newPrimaryButton("TRANSCRIBE", p.onTranscribe)

	p.progress = newThinProgress()

	p.statusText = canvas.NewText("READY", colMuted)
	p.statusText.TextSize = textBody
	p.statusText.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}

	p.timerText = canvas.NewText("", colMuted)
	p.timerText.TextSize = textBody
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

	p.container = p.dropArea
}
