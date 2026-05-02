package gui

import (
	"runtime"

	"fyne.io/fyne/v2"

	"github.com/asolopovas/wt/internal/gui/assets"
	"github.com/asolopovas/wt/internal/gui/decor"
	"github.com/asolopovas/wt/internal/gui/transcribe"
)

type (
	pointerButton = decor.PointerButton
	pointerSelect = decor.PointerSelect
	limitSelect   = decor.LimitSelect
)

var (
	newPointerButton         = decor.NewPointerButton
	newPointerButtonWithIcon = decor.NewPointerButtonWithIcon
	newPointerSelect         = decor.NewPointerSelect
	newLimitSelect           = decor.NewLimitSelect
)

const (
	notifyInfo    = decor.NotifyInfo
	notifySuccess = decor.NotifySuccess
)

var monoBoldStyle = decor.MonoBoldStyle

var (
	showNotice  = decor.ShowNotice
	showError   = decor.ShowError
	showConfirm = decor.ShowConfirm
)

var (
	appIcon           = assets.AppIcon
	playIconResource  = assets.PlayIcon
	pauseIconResource = assets.PauseIcon
	downloadIcon      = assets.DownloadIcon
)

var validModels = func() []string {
	m := []string{"tiny", "base", "small", "medium"}
	if runtime.GOOS != "android" {
		m = append(m, "large-v3")
	}
	return append(m, "turbo")
}()

func attachLibrary(p *transcribe.Panel, h transcribe.History) {
	if p.LibraryHost == nil {
		return
	}
	p.LibraryHost.Objects = []fyne.CanvasObject{h.Container()}
	p.LibraryHost.Refresh()
}
