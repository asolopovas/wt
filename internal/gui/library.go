package gui

import (
	"fyne.io/fyne/v2"

	"github.com/asolopovas/wt/internal/gui/transcribe"
)

func attachLibrary(p *transcribe.Panel, h transcribe.History) {
	if p.LibraryHost == nil {
		return
	}
	p.LibraryHost.Objects = []fyne.CanvasObject{h.Container()}
	p.LibraryHost.Refresh()
}
