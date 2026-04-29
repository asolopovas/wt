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
	uploadIcon := canvas.NewText("⬆", colPrimary)
	uploadIcon.TextSize = 24
	uploadIcon.Alignment = fyne.TextAlignCenter

	p.dropText = canvas.NewText("DROP AUDIO FILES HERE OR CLICK TO BROWSE", colMuted)
	p.dropText.Alignment = fyne.TextAlignCenter
	p.dropText.TextSize = 11

	innerDash := canvas.NewRectangle(colSurfMid)
	innerDash.StrokeColor = colPrimaryGhost
	innerDash.StrokeWidth = 1

	dropTap := newTappableArea(p.onBrowse)
	innerContent := container.NewVBox(
		container.NewCenter(uploadIcon),
		container.NewCenter(p.dropText),
	)
	innerZone := container.NewStack(innerDash, container.NewPadded(innerContent), dropTap)

	chipsFlow := newFlowLayout(8)
	p.fileChips = container.New(chipsFlow)
	chipsFlow.setParent(p.fileChips)

	outerBg := canvas.NewRectangle(colSurfLowest)
	outerBg.StrokeColor = colGhostBorder
	outerBg.StrokeWidth = 1

	dropContent := container.NewVBox(innerZone, p.fileChips)
	p.dropArea = container.NewStack(outerBg, container.NewPadded(dropContent))

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

	p.logText = widget.NewRichText()
	p.logText.Wrapping = fyne.TextWrapWord
	appendLogInit(p.logText)

	p.logScroll = container.NewVScroll(p.logText)

	p.copyLogBtn = newPointerButtonWithIcon("", theme.ContentCopyIcon(), p.onCopyLog)
	p.copyLogBtn.Importance = widget.LowImportance

	p.clearLogBtn = newPointerButtonWithIcon("", theme.HistoryIcon(), p.onClearLog)
	p.clearLogBtn.Importance = widget.LowImportance

	p.autoScroll.Store(true)
	p.autoBtn = newPointerButtonWithIcon("", theme.MoveDownIcon(), nil)
	p.autoBtn.Importance = widget.HighImportance
	p.autoBtn.OnTapped = p.toggleAutoScroll

	logPanel := buildLogPanel(p.logScroll, p.statsLine, nil, nil)

	p.container = container.New(newResponsiveColumns(8), p.dropArea, logPanel)
}
