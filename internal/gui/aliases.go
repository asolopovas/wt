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

// displayNameToWhisperSize maps a unified-dropdown display name back to
// a whisper size string for cfg.Model. For non-whisper picks (Parakeet,
// SenseVoice, ...) the dropdown choice is captured by the manager's
// FamilyASR active selection — cfg.Model keeps the previous whisper size
// so the user's whisper preference is preserved when they toggle back.
func displayNameToWhisperSize(displayName, fallback string) string {
	for size, id := range whisperSizeToID {
		if e, ok := models.ByID(id); ok && e.DisplayName == displayName {
			return size
		}
	}
	// If displayName matches an existing whisper size literal (legacy
	// persisted value), pass it through.
	if _, ok := whisperSizeToID[displayName]; ok {
		return displayName
	}
	return fallback
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

// dropdownModels returns the unified list of installed transcription
// engine display names — whisper sizes followed by sherpa-backed ASR
// engines (Parakeet, SenseVoice, etc.). Both Settings→Models and the
// Transcode tab MODEL dropdown read from this single source of truth.
//
// currentSelection is preserved as a fallback option even if not currently
// installed (so the user sees what's persisted in settings rather than a
// silent jump to a different model).
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

// defaultWhisperDisplayName is the display name shown when nothing is
// installed yet (catalog default for the platform).
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

// dropdownDiarizers returns installed FamilyDiarizer display names for
// the Transcode-tab DIARIZER dropdown.
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
