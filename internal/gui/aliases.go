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
	"tiny":  "whisper-tiny",
	"small": "whisper-small",
	"turbo": "whisper-turbo",
}

func displayNameToWhisperSize(displayName, fallback string) string {
	for size, id := range whisperSizeToID {
		if e, ok := models.ByID(id); ok && e.DisplayName == displayName {
			return size
		}
	}

	if _, ok := whisperSizeToID[displayName]; ok {
		return displayName
	}
	return fallback
}

func allowedModelSizes() []string {
	return []string{"tiny", "small", "turbo"}
}

func defaultWhisperSize() string {
	if runtime.GOOS == "android" {
		return "tiny"
	}
	return "turbo"
}

func dropdownModels(currentSelection string) []string {
	mgr := models.NewManager()
	opts := pickerLabels(transcriptionPickerOptions(mgr))
	if currentSelection != "" && !slices.Contains(opts, currentSelection) {
		opts = append(opts, currentSelection)
	}
	if len(opts) == 0 {
		opts = []string{defaultWhisperDisplayName()}
	}
	return opts
}

func defaultWhisperDisplayName() string {
	id := "whisper-turbo"
	if runtime.GOOS == "android" {
		id = "whisper-tiny"
	}
	if e, ok := models.ByID(id); ok {
		return e.DisplayName
	}
	return id
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
