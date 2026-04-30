package gui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (p *transcribePanel) logTextString() string {
	var sb strings.Builder
	for _, seg := range p.logText.Segments {
		if ts, ok := seg.(*widget.TextSegment); ok {
			sb.WriteString(ts.Text)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func (p *transcribePanel) onShareLog() {
	content := p.logTextString()
	if strings.TrimSpace(content) == "" {
		return
	}
	defaultName := "wt-log-" + time.Now().Format("20060102-150405") + ".txt"
	saveDialog := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil || w == nil {
			return
		}
		defer func() { _ = w.Close() }()
		_, _ = w.Write([]byte(content))
	}, p.window)
	saveDialog.SetFileName(defaultName)
	if home, herr := os.UserHomeDir(); herr == nil {
		if uri, lerr := storage.ListerForURI(storage.NewFileURI(filepath.Clean(home))); lerr == nil {
			saveDialog.SetLocation(uri)
		}
	}
	saveDialog.Show()
}

func (p *transcribePanel) openLogDialog() {
	rt := widget.NewRichText()
	rt.Wrapping = fyne.TextWrapWord
	for _, seg := range p.logText.Segments {
		if ts, ok := seg.(*widget.TextSegment); ok {
			rt.Segments = append(rt.Segments, &widget.TextSegment{
				Text:  ts.Text,
				Style: ts.Style,
			})
		}
	}
	scroll := container.NewVScroll(rt)
	p.logMirror = rt
	p.logMirrorScr = scroll

	cs := p.window.Canvas().Size()
	const (
		sideMargin   = 12
		topMargin    = 24
		bottomMargin = 80
	)
	w := cs.Width - sideMargin*2
	h := cs.Height - topMargin - bottomMargin
	if w < 1 {
		w = cs.Width
	}
	if h < 1 {
		h = cs.Height
	}

	var pop *widget.PopUp

	copyBtn := newPointerButtonWithIcon("", theme.ContentCopyIcon(), p.onCopyLog)
	copyBtn.Importance = widget.HighImportance

	shareBtn := newPointerButtonWithIcon("", theme.MailForwardIcon(), func() {
		p.logMirror = nil
		p.logMirrorScr = nil
		if pop != nil {
			pop.Hide()
		}
		p.onShareLog()
	})
	shareBtn.Importance = widget.HighImportance

	closeBtn := newPointerButtonWithIcon("", theme.CancelIcon(), func() {
		p.logMirror = nil
		p.logMirrorScr = nil
		if pop != nil {
			pop.Hide()
		}
	})
	closeBtn.Importance = widget.LowImportance

	floatGap := canvas.NewRectangle(transparent)
	floatGap.SetMinSize(fyne.NewSize(0, 12))
	floatBar := container.NewHBox(copyBtn, shareBtn, closeBtn)
	floatRow := container.NewBorder(nil, nil, nil, floatBar, floatGap)

	body := container.NewBorder(floatRow, nil, nil, nil, scroll)

	bg := canvas.NewRectangle(colSurfLowest)
	frame := canvas.NewRectangle(transparent)
	frame.StrokeColor = colDialogBorder
	frame.StrokeWidth = 1
	topPad := canvas.NewRectangle(transparent)
	topPad.SetMinSize(fyne.NewSize(0, 8))
	bottomPad := canvas.NewRectangle(transparent)
	bottomPad.SetMinSize(fyne.NewSize(0, 8))
	leftPad := canvas.NewRectangle(transparent)
	leftPad.SetMinSize(fyne.NewSize(4, 0))
	rightPad := canvas.NewRectangle(transparent)
	rightPad.SetMinSize(fyne.NewSize(4, 0))
	bordered := container.NewStack(bg, frame, container.NewBorder(topPad, bottomPad, leftPad, rightPad, body))

	pop = widget.NewModalPopUp(bordered, p.window.Canvas())
	pop.Resize(fyne.NewSize(w, h))
	pop.Move(fyne.NewPos(sideMargin, topMargin))
	pop.Show()
}
