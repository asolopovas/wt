//go:build !android

package gui

import (
	"fyne.io/fyne/v2/container"
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

	p.buildSharedControls()

	p.container = p.dropArea
}
