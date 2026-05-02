package gui

import (
	"runtime"
	"slices"

	"fyne.io/fyne/v2"

	"github.com/asolopovas/wt/internal/gui/assets"
	"github.com/asolopovas/wt/internal/gui/decor"
	"github.com/asolopovas/wt/internal/gui/transcribe"
	"github.com/asolopovas/wt/internal/models"
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

var monoBoldStyle = decor.MonoBoldStyle

var (
	showError   = decor.ShowError
	showConfirm = decor.ShowConfirm
)

var (
	appIcon           = assets.AppIcon
	playIconResource  = assets.PlayIcon
	pauseIconResource = assets.PauseIcon
	editAudioIcon     = assets.EditAudioIcon
	downloadIcon      = assets.DownloadIcon
)

var whisperSizeToID = map[string]string{
	"tiny":     "whisper-tiny",
	"base":     "whisper-base",
	"small":    "whisper-small",
	"medium":   "whisper-medium",
	"large-v3": "whisper-large-v3",
	"turbo":    "whisper-turbo",
}

func allowedModelSizes() []string {
	m := []string{"tiny", "base", "small", "medium"}
	if runtime.GOOS != "android" {
		m = append(m, "large-v3")
	}
	return append(m, "turbo")
}

func defaultWhisperSize() string {
	if runtime.GOOS == "android" {
		return "tiny"
	}
	return "turbo"
}

func installedModelSizes() []string {
	mgr := models.NewManager()
	var out []string
	for _, sz := range allowedModelSizes() {
		id, ok := whisperSizeToID[sz]
		if !ok {
			continue
		}
		if mgr.Status(id) == models.StatusInstalled {
			out = append(out, sz)
		}
	}
	return out
}

func dropdownModels(currentSelection string) []string {
	opts := installedModelSizes()
	if currentSelection != "" && slices.Contains(allowedModelSizes(), currentSelection) && !slices.Contains(opts, currentSelection) {
		opts = append(opts, currentSelection)
	}
	if len(opts) == 0 {
		opts = []string{defaultWhisperSize()}
	}
	return opts
}

func attachLibrary(p *transcribe.Panel, h transcribe.History) {
	if p.LibraryHost == nil {
		return
	}
	p.LibraryHost.Objects = []fyne.CanvasObject{h.Container()}
	p.LibraryHost.Refresh()
}
