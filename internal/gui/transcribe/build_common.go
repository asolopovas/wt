package transcribe

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (p *Panel) buildSharedControls() {
	p.clearBtn = newPointerButton("CLEAR ALL", p.onClear)
	p.clearCacheBtn = newPointerButton("CLEAR CACHE", p.onClearCache)
	p.TranscribeBtn = newPrimaryButton("TRANSCRIBE", p.onTranscribe)

	p.Progress = newThinProgress()

	p.StatusText = canvas.NewText("READY", colMuted)
	p.StatusText.TextSize = textBody
	p.StatusText.TextStyle = monoBoldStyle

	p.TimerText = canvas.NewText("", colMuted)
	p.TimerText.TextSize = textBody
	p.TimerText.TextStyle = monoBoldStyle
	p.TimerText.Alignment = fyne.TextAlignTrailing

	p.StatsLine = widget.NewLabel("")
	p.StatsLine.TextStyle = fyne.TextStyle{Monospace: true}

	p.LogEntry = widget.NewMultiLineEntry()
	p.LogEntry.TextStyle = fyne.TextStyle{Monospace: true}
	p.LogEntry.Wrapping = fyne.TextWrapWord
	p.LogEntry.Disable()

	p.CopyLogBtn = newPointerButtonWithIcon("", theme.ContentCopyIcon(), p.onCopyLog)
	p.CopyLogBtn.Importance = widget.LowImportance

	p.ClearLogBtn = newPointerButtonWithIcon("", theme.HistoryIcon(), p.onClearLog)
	p.ClearLogBtn.Importance = widget.LowImportance

	p.autoScroll.Store(true)
	p.AutoBtn = newPointerButtonWithIcon("", theme.MoveDownIcon(), nil)
	p.AutoBtn.Importance = widget.HighImportance
	p.AutoBtn.OnTapped = p.toggleAutoScroll

	appendLogInit(p)
}
