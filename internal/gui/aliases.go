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

func defaultASRModelID() string {
	if runtime.GOOS == "android" {
		return "sherpa-whisper-tiny.en"
	}
	return "sherpa-whisper-turbo"
}

func defaultASRDisplayName() string {
	if e, ok := models.ByID(defaultASRModelID()); ok {
		return e.DisplayName
	}
	return defaultASRModelID()
}

func displayNameToModelID(displayName, fallback string) string {
	mgr := models.NewManager()
	for _, opt := range transcriptionPickerOptions(mgr) {
		if opt.DisplayName == displayName {
			return opt.ID
		}
	}
	if _, ok := models.ByID(displayName); ok {
		return displayName
	}
	return fallback
}

func dropdownModels(currentSelection string) []string {
	mgr := models.NewManager()
	opts := pickerLabels(transcriptionPickerOptions(mgr))
	if currentSelection != "" && !slices.Contains(opts, currentSelection) {
		opts = append(opts, currentSelection)
	}
	if len(opts) == 0 {
		opts = []string{defaultASRDisplayName()}
	}
	return opts
}

func dropdownDiarizers(currentSelection string) []string {
	mgr := models.NewManager()
	opts := pickerLabels(diarizerPickerOptions(mgr))
	if currentSelection != "" && !slices.Contains(opts, currentSelection) {
		opts = append(opts, currentSelection)
	}
	if len(opts) == 0 {
		if e, ok := models.ByID("diar-titanet-large"); ok {
			opts = []string{e.DisplayName}
		}
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
