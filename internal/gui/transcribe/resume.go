package transcribe

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/asolopovas/wt/internal/transcriber"
)

type resumeChoice int

const (
	resumeFresh resumeChoice = iota
	resumeYes
	resumeAbort
)

func (p *Panel) promptResume(filename string, resumeAt time.Duration, segCount int) resumeChoice {
	if p.window == nil {
		return resumeYes
	}

	body := widget.NewLabel(fmt.Sprintf(
		"%s has a partial transcript saved (%d segments, up to %s).\nResume from there or start over?",
		filename, segCount, transcriber.FormatHMS(resumeAt),
	))
	body.Wrapping = fyne.TextWrapWord

	ch := make(chan resumeChoice, 1)
	send := func(c resumeChoice) {
		select {
		case ch <- c:
		default:
		}
	}

	fyne.Do(func() {
		showDialog(dialogConfig{
			Parent: p.window,
			Title:  "RESUME TRANSCRIPTION?",
			Body:   body,
			Actions: []dialogAction{
				{Label: "CANCEL", Kind: kindSecondary, OnTap: func() { send(resumeAbort) }},
				{Label: "START OVER", Kind: kindSecondary, OnTap: func() { send(resumeFresh) }},
				{Label: "RESUME", Kind: kindPrimary, OnTap: func() { send(resumeYes) }},
			},
		})
	})

	return <-ch
}
