//go:build !android

package transcribe

import (
	"fyne.io/fyne/v2/container"
)

var (
	audioExtensionList = baseAudioExtensions
	audioExtensions    = extensionSet(audioExtensionList)
)

func (p *Panel) build() {
	chipsFlow := newFlowLayout(spaceLG)
	p.fileChips = container.New(chipsFlow)
	chipsFlow.setParent(p.fileChips)

	p.LibraryHost = container.NewStack()
	p.dropArea = container.NewStack(newPanelBackground(), container.NewPadded(p.LibraryHost))

	p.buildSharedControls()

	p.Container = p.dropArea
}
