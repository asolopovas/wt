// Package preview — shared text-viewer modal.
//
// ShowText renders monospace text content in a high-contrast, scrollable
// modal that matches the transcription preview chrome (surfaceRaised
// background, mono-bold rows, CLOSE + COPY buttons). Used by:
//   - Settings → VIEW LOG (wt.log tail)
//   - Future: about/license dialogs, debug dumps, etc.
//
// Single source of truth so every read-only text modal in the app looks
// and behaves the same. Add new use sites by calling ShowText, never
// hand-rolling another widget.NewModalPopUp.
package preview

import (
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/asolopovas/wt/internal/gui/decor"
)

// TextViewerOpts configures a ShowText call. Only Window and Body are
// required; other fields default sensibly.
type TextViewerOpts struct {
	Window  fyne.Window
	Title   string // header caption (e.g. "wt.log"); empty = no header
	Body    string // content to display, displayed verbatim
	Mono    bool   // monospace font (default true via zero-value flip)
	OnClose func()
}

// ShowText pops a modal text viewer. Returns a hide() callback. The
// modal includes a COPY button that puts Body on the system clipboard
// with a tick-icon confirmation, and a CLOSE button. Selection / scroll
// inside the body work the same as the transcription preview.
func ShowText(opts TextViewerOpts) func() {
	if opts.Window == nil {
		return func() {}
	}
	if opts.Body == "" {
		opts.Body = "(empty)"
	}

	// High-contrast text rendering: split into lines and use canvas.Text
	// with decor.TextPrimary so the result reads exactly like the
	// transcription preview. widget.MultiLineEntry().Disable() renders
	// pale-gray on dark which is illegible \u2014 avoid that path.
	// Default: monospace. Mono field reserved for opt-out future use.
	style := fyne.TextStyle{Monospace: true}
	_ = opts.Mono
	rows := make([]fyne.CanvasObject, 0, strings.Count(opts.Body, "\n")+1)
	for _, line := range strings.Split(opts.Body, "\n") {
		t := canvas.NewText(line, decor.TextPrimary)
		t.TextSize = 12
		t.TextStyle = style
		rows = append(rows, t)
	}
	content := container.NewVBox(rows...)
	scroll := container.NewScroll(content)
	scroll.SetMinSize(ScrollMinSize())
	bg := canvas.NewRectangle(decor.SurfaceRaised)
	scrollPanel := container.NewStack(bg, scroll)

	// COPY button: tick-confirmation pattern matching the transcription
	// preview. Imports stayed identical to keep the visual familiar.
	copyBtn := decor.NewPointerButtonWithIcon("", theme.ContentCopyIcon(), nil)
	copyBtn.Importance = widget.LowImportance
	copyBtn.OnTapped = func() {
		fyne.CurrentApp().Clipboard().SetContent(opts.Body)
		copyBtn.SetIcon(theme.ConfirmIcon())
		go func() {
			time.Sleep(900 * time.Millisecond)
			fyne.Do(func() { copyBtn.SetIcon(theme.ContentCopyIcon()) })
		}()
	}
	copyRow := container.NewGridWithColumns(3,
		layout.NewSpacer(),
		layout.NewSpacer(),
		container.NewHBox(layout.NewSpacer(), copyBtn),
	)

	var hide func()
	closeBtn := decor.NewSecondaryButton("CLOSE", func() {
		if hide != nil {
			hide()
		}
	})
	bottomGap := canvas.NewRectangle(decor.Transparent)
	bottomGap.SetMinSize(fyne.NewSize(0, BottomInset()))
	actionRow := container.NewVBox(copyRow, decor.WrapAction(closeBtn), bottomGap)

	var top fyne.CanvasObject
	if opts.Title != "" {
		topGap := canvas.NewRectangle(decor.Transparent)
		topGap.SetMinSize(fyne.NewSize(0, TopInset()))
		titleText := canvas.NewText(opts.Title, decor.TextMuted)
		titleText.TextSize = 12
		titleText.TextStyle = decor.MonoBoldStyle
		top = container.NewVBox(topGap, titleText)
	}

	body := container.NewBorder(top, actionRow, nil, nil, scrollPanel)
	hide = ShowTranscript(opts.Title, body, opts.Window, opts.OnClose)
	return hide
}
