//go:build android

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"github.com/asolopovas/wt/internal/gui/transcribe"
)

// playerDock is a no-op stub on Android; the platform plays via MediaPlayer
// directly from history rows and screen real estate is too tight for a
// waveform editor. Keeping the type so layout code compiles cross-platform.
type playerDock struct{ root *fyne.Container }

func newPlayerDock(_ fyne.Window, _ *transcribe.Panel) *playerDock {
	return &playerDock{root: container.NewWithoutLayout()}
}

func (d *playerDock) Container() fyne.CanvasObject       { return d.root }
func (d *playerDock) Load(_, _, _ string, _ bool)        {}
func (d *playerDock) Close()                             {}
