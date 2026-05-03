package gui

import (
	"fmt"
	"os"
	"runtime"
	"slices"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/models"
	"github.com/asolopovas/wt/internal/transcriber/cache"
)

var languages = []string{
	"auto", "en", "zh", "de", "es", "ru", "ko", "fr", "ja", "pt",
	"tr", "pl", "ca", "nl", "ar", "sv", "it", "id", "hi", "fi",
	"vi", "he", "uk", "el", "ms", "cs", "ro", "da", "hu", "ta",
	"no", "th", "ur", "hr", "bg", "lt", "la", "mi", "ml", "cy",
}

// speakerOptions is capped at 4 because the active diarizer
// (NeMo Sortformer 4spk) is hardcoded at 4 speakers; passing >4 is
// silently ignored upstream (see scripts/diarize.py:110). "auto" lets
// the model decide.
var speakerOptions = []string{
	"auto", "2", "3", "4",
}

var cacheExpiryOptions = []string{"7", "30", "90", "365", "never"}

type settingsPanel struct {
	cfg    shared.Config
	window fyne.Window

	modelSelect    *pointerSelect
	langSelect     *limitSelect
	deviceSelect   *pointerSelect
	threadsSelect  *limitSelect
	speakersSelect *pointerSelect
	expirySelect   *pointerSelect

	modelMirrors    []*pointerSelect
	diarizerMirrors []*pointerSelect
	onModelsChanged func()
	langMirrors     []*limitSelect
	speakersMirrors []*pointerSelect
	noDiarizeBtn    *pointerButton
	noDiarizeState  bool
	debugBtn        *pointerButton
	debugState      bool
	saveBtn         *pointerButton

	models       *modelsSection
	settingsGrid fyne.CanvasObject
	actionRow    fyne.CanvasObject

	onCacheCleared func()

	container fyne.CanvasObject
}

func newSettingsPanel(cfg shared.Config, window fyne.Window) *settingsPanel {
	p := &settingsPanel{cfg: cfg, window: window, noDiarizeState: cfg.NoDiarize}
	p.build()
	return p
}

func (p *settingsPanel) build() {
	persist := func(string) { p.persist() }

	modelOpts := dropdownModels("")
	p.modelSelect = newPointerSelect(modelOpts, p.onModelChanged)
	// Initial dropdown selection: prefer the manager's active transcription
	// entry (covers the user picking from Settings→Models). Fall back to
	// translating the legacy cfg.Model size string to a display name.
	mgr := models.NewManager()
	selected := activeTranscriptionDisplayName(transcriptionPickerOptions(mgr), mgr)
	if selected == "" {
		if id, ok := whisperSizeToID[p.cfg.Model]; ok {
			if e, ok2 := models.ByID(id); ok2 {
				selected = e.DisplayName
			}
		}
	}
	if selected == "" || !slices.Contains(modelOpts, selected) {
		selected = modelOpts[0]
	}
	p.modelSelect.Selected = selected
	p.modelMirrors = append(p.modelMirrors, p.modelSelect)

	langLabel := "auto"
	if p.cfg.Language != "" {
		langLabel = p.cfg.Language
	}
	p.langSelect = newLimitSelect(languages, 300, p.onLangChanged)
	p.langSelect.Inner.Selected = langLabel
	p.langMirrors = append(p.langMirrors, p.langSelect)
	// Constrain to the active engine's whitelist after both the model and
	// language widgets are wired (filterLanguageOptions reads the
	// manager). Safe to call before mirrors exist; refreshLanguageOptions
	// is a no-op when langMirrors is empty.
	defer p.refreshLanguageOptions()

	p.deviceSelect = newPointerSelect([]string{"auto", "cuda", "cpu"}, persist)
	p.deviceSelect.Selected = p.cfg.Device

	maxThreads := runtime.NumCPU()
	threadOpts := make([]string, maxThreads)
	for i := range maxThreads {
		threadOpts[i] = strconv.Itoa(i + 1)
	}
	p.threadsSelect = newLimitSelect(threadOpts, 300, persist)
	p.threadsSelect.Inner.Selected = strconv.Itoa(p.cfg.Threads)

	p.speakersSelect = newPointerSelect(speakerOptions, p.onSpeakersChanged)
	spkSel := "auto"
	if p.cfg.Speakers > 0 {
		spkSel = strconv.Itoa(p.cfg.Speakers)
	}
	p.speakersSelect.Selected = spkSel
	p.speakersMirrors = append(p.speakersMirrors, p.speakersSelect)

	p.expirySelect = newPointerSelect(cacheExpiryOptions, persist)
	if p.cfg.CacheExpiryDays <= 0 {
		p.expirySelect.Selected = "never"
	} else {
		p.expirySelect.Selected = strconv.Itoa(p.cfg.CacheExpiryDays)
	}

	p.noDiarizeBtn = newPointerButton("", p.onToggleDiarize)
	p.updateDiarizeLabel()

	p.debugBtn = newPointerButton("", p.onToggleDebug)
	p.updateDebugLabel()

	p.saveBtn = newPrimaryButton("SAVE", p.onSave)

	clearCacheBtn := newSecondaryButton("CLEAR CACHE", p.onClearCache)
	clearTranscriptsBtn := newSecondaryButton("CLEAR TEXT", p.onClearTranscripts)

	settingsGrid := container.NewGridWithColumns(2,
		newFormField("MODEL", p.modelSelect),
		newFormField("LANGUAGE", p.langSelect),
		newFormField("DEVICE", p.deviceSelect),
		newFormField("THREADS", p.threadsSelect),
		newFormField("SPEAKERS", p.speakersSelect),
		newFormField("CACHE EXPIRY (DAYS)", p.expirySelect),
	)

	toggleRow := container.NewGridWithColumns(2,
		wrapGhost(p.noDiarizeBtn),
		wrapGhost(p.debugBtn),
	)
	clearRow := container.NewGridWithColumns(2,
		wrapAction(clearCacheBtn),
		wrapAction(clearTranscriptsBtn),
	)
	p.actionRow = container.NewVBox(
		toggleRow,
		vGap(spaceSM),
		clearRow,
		vGap(spaceSM),
		wrapAction(p.saveBtn),
	)

	p.models = newModelsSection(p.window)
	p.models.onChanged = p.refreshModelOptions
	p.settingsGrid = settingsGrid

	p.container = container.NewVBox(
		settingsGrid,
		vGap(spaceMD),
		p.actionRow,
		vGap(spaceMD),
		newSectionDivider(),
		p.models.container,
		layout.NewSpacer(),
	)
}

func (p *settingsPanel) onToggleDiarize() {
	p.noDiarizeState = !p.noDiarizeState
	p.updateDiarizeLabel()
	p.persist()
}

func (p *settingsPanel) updateDiarizeLabel() {
	if p.noDiarizeState {
		p.noDiarizeBtn.SetText("DIARIZE OFF")
		p.noDiarizeBtn.Importance = widget.DangerImportance
	} else {
		p.noDiarizeBtn.SetText("DIARIZE ON")
		p.noDiarizeBtn.Importance = widget.SuccessImportance
	}
}

func (p *settingsPanel) onToggleDebug() {
	p.debugState = !p.debugState
	p.updateDebugLabel()
}

func (p *settingsPanel) updateDebugLabel() {
	if p.debugState {
		p.debugBtn.SetText("DEBUG ON")
		p.debugBtn.Importance = widget.WarningImportance
	} else {
		p.debugBtn.SetText("DEBUG OFF")
		p.debugBtn.Importance = widget.LowImportance
	}
}

func (p *settingsPanel) Debug() bool {
	return p.debugState
}

func (p *settingsPanel) onClearCache() {
	cacheDir := shared.CacheDir()
	if err := os.RemoveAll(cacheDir); err != nil {
		showError(p.window, fmt.Errorf("clearing cache: %w", err))
		return
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		showError(p.window, fmt.Errorf("recreating cache dir: %w", err))
		return
	}
}

func (p *settingsPanel) onClearTranscripts() {
	showConfirm(p.window, "Clear transcripts",
		"Remove all cached transcripts? This cannot be undone.",
		func() {
			if err := cache.Clear(); err != nil {
				showError(p.window, err)
				return
			}
			if p.onCacheCleared != nil {
				p.onCacheCleared()
			}
		})
}

func (p *settingsPanel) cacheExpiryDays() int {
	sel := p.expirySelect.Selected
	if sel == "never" || sel == "" {
		return 0
	}
	n, err := strconv.Atoi(sel)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func (p *settingsPanel) onSave() {
	if err := p.writeConfig(); err != nil {
		showError(p.window, err)
		return
	}
}

func (p *settingsPanel) persist() {
	if err := p.writeConfig(); err != nil {
		fyne.LogError("auto-saving settings", err)
	}
}

func (p *settingsPanel) writeConfig() error {
	cfg, err := shared.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// p.modelSelect.Selected is now a display name ("Whisper large-v3-turbo"
	// or "Parakeet TDT 0.6B v2 (English)"). Translate back to a whisper
	// size string for cfg.Model so legacy code paths (e.g. whisper-cpp
	// model loader) keep working. ASR engine routing is driven by the
	// manager's active state — cfg.Model is only relevant for whisper.
	cfg.Model = displayNameToWhisperSize(p.modelSelect.Selected, p.cfg.Model)
	cfg.Language = p.Language()
	cfg.Device = p.deviceSelect.Selected
	cfg.Threads = p.Threads()
	cfg.Speakers = p.Speakers()
	cfg.NoDiarize = p.noDiarizeState
	cfg.CacheExpiryDays = p.cacheExpiryDays()

	if err := shared.Save(cfg); err != nil {
		return fmt.Errorf("saving settings: %w", err)
	}
	p.cfg = cfg
	return nil
}

func (p *settingsPanel) ModelSize() string {
	return displayNameToWhisperSize(p.modelSelect.Selected, p.cfg.Model)
}

func (p *settingsPanel) Language() string {
	lang := p.langSelect.Inner.Selected
	if lang == "auto" {
		return ""
	}
	return lang
}

func (p *settingsPanel) Device() string {
	return p.deviceSelect.Selected
}

func (p *settingsPanel) Threads() int {
	n, err := strconv.Atoi(p.threadsSelect.Inner.Selected)
	if err != nil || n < 1 {
		return runtime.NumCPU()
	}
	return n
}

func (p *settingsPanel) Speakers() int {
	sel := p.speakersSelect.Selected
	if sel == "auto" || sel == "" {
		return 0
	}
	n, err := strconv.Atoi(sel)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func (p *settingsPanel) NoDiarize() bool {
	return p.noDiarizeState
}

func (p *settingsPanel) onModelChanged(v string) {
	for _, m := range p.modelMirrors {
		if m.Selected != v {
			m.Selected = v
			m.Refresh()
		}
	}
	// Sync the change into the models manager so Settings→Models reflects
	// the dropdown choice and runner.go's engine resolver picks the right
	// entry. Lookup is by display name (the option label).
	mgr := models.NewManager()
	opts := transcriptionPickerOptions(mgr)
	if id := pickerByDisplayName(opts, v); id != "" {
		_ = setActiveTranscription(mgr, id)
		if p.onModelsChanged != nil {
			p.onModelsChanged()
		}
	}
	p.refreshLanguageOptions()
	p.persist()
}

// refreshLanguageOptions recomputes the LANGUAGE dropdown options to
// match the active engine's whitelist (e.g. Parakeet -> ["en"] only).
// Whisper / multilingual engines see the full list.
func (p *settingsPanel) refreshLanguageOptions() {
	if len(p.langMirrors) == 0 {
		return
	}
	mgr := models.NewManager()
	allowed := supportedLanguagesForActive(mgr)
	current := p.langSelect.Inner.Selected
	opts, selected := filterLanguageOptions(languages, allowed, current)
	for _, m := range p.langMirrors {
		m.Inner.Options = opts
		m.Inner.Selected = selected
		// Disable the dropdown when there's only one valid choice (no
		// point letting the user open a single-item menu).
		if len(opts) <= 1 {
			m.Inner.Disable()
		} else {
			m.Inner.Enable()
		}
		m.Inner.Refresh()
	}
	if current != selected {
		p.persist()
	}
}

func (p *settingsPanel) onDiarizerChanged(v string) {
	for _, m := range p.diarizerMirrors {
		if m.Selected != v {
			m.Selected = v
			m.Refresh()
		}
	}
	mgr := models.NewManager()
	opts := diarizerPickerOptions(mgr)
	if id := pickerByDisplayName(opts, v); id != "" {
		_ = mgr.SetActive(id)
		if p.onModelsChanged != nil {
			p.onModelsChanged()
		}
	}
	p.persist()
}

func (p *settingsPanel) newDiarizerSelectMirror() *pointerSelect {
	mgr := models.NewManager()
	opts := dropdownDiarizers("")
	selected := activeDiarizerDisplayName(diarizerPickerOptions(mgr), mgr)
	s := newPointerSelect(opts, p.onDiarizerChanged)
	s.Selected = selected
	p.diarizerMirrors = append(p.diarizerMirrors, s)
	return s
}

// refreshDiarizerOptions repopulates all diarizer mirrors after
// install/delete/active changes from Settings→Models.
func (p *settingsPanel) refreshDiarizerOptions() {
	mgr := models.NewManager()
	opts := dropdownDiarizers("")
	selected := activeDiarizerDisplayName(diarizerPickerOptions(mgr), mgr)
	for _, m := range p.diarizerMirrors {
		m.Options = opts
		m.Selected = selected
		m.Refresh()
	}
}

func (p *settingsPanel) onLangChanged(v string) {
	for _, m := range p.langMirrors {
		if m.Inner.Selected != v {
			m.Inner.Selected = v
			m.Inner.Refresh()
		}
	}
	p.persist()
}

func (p *settingsPanel) onSpeakersChanged(v string) {
	for _, m := range p.speakersMirrors {
		if m.Selected != v {
			m.Selected = v
			m.Refresh()
		}
	}
	p.persist()
}

func (p *settingsPanel) newModelSelectMirror() *pointerSelect {
	s := newPointerSelect(dropdownModels(p.modelSelect.Selected), p.onModelChanged)
	s.Selected = p.modelSelect.Selected
	p.modelMirrors = append(p.modelMirrors, s)
	return s
}

func (p *settingsPanel) refreshModelOptions() {
	// Pull the active transcription entry from the manager so changes made
	// in Settings→Models tap-to-activate are reflected in the dropdown.
	mgr := models.NewManager()
	pickerOpts := transcriptionPickerOptions(mgr)
	opts := dropdownModels(p.modelSelect.Selected)
	selected := activeTranscriptionDisplayName(pickerOpts, mgr)
	if selected == "" {
		selected = p.modelSelect.Selected
	}
	for _, m := range p.modelMirrors {
		m.Options = opts
		if selected != "" && slices.Contains(opts, selected) {
			m.Selected = selected
		} else if m.Selected == "" || !slices.Contains(opts, m.Selected) {
			m.Selected = opts[0]
		}
		m.Refresh()
	}
	p.refreshDiarizerOptions()
	p.refreshLanguageOptions()
}

func (p *settingsPanel) newLangSelectMirror() *limitSelect {
	s := newLimitSelect(languages, 300, p.onLangChanged)
	s.Inner.Selected = p.langSelect.Inner.Selected
	p.langMirrors = append(p.langMirrors, s)
	return s
}

func (p *settingsPanel) newSpeakersSelectMirror() *pointerSelect {
	s := newPointerSelect(speakerOptions, p.onSpeakersChanged)
	s.Selected = p.speakersSelect.Selected
	p.speakersMirrors = append(p.speakersMirrors, s)
	return s
}
