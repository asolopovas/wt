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

	"github.com/asolopovas/wt/internal/gui/preview"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/models"
	"github.com/asolopovas/wt/internal/transcriber/cache"
)

var languages = allLanguageCodes()

var speakerOptions = []string{
	"auto", "2", "3", "4",
}

var cacheExpiryOptions = []string{"7", "30", "90", "365", "never"}

var logRetentionOptions = []string{"24 hours", "7 days", "30 days", "forever"}

func logRetentionLabelToDays(s string) int {
	switch s {
	case "24 hours":
		return 1
	case "7 days":
		return 7
	case "30 days":
		return 30
	case "forever":
		return 0
	}
	return 1
}

func logRetentionDaysToLabel(d int) string {
	switch d {
	case 0:
		return "forever"
	case 7:
		return "7 days"
	case 30:
		return "30 days"
	}
	return "24 hours"
}

type settingsPanel struct {
	cfg    shared.Config
	window fyne.Window

	modelSelect     *pointerSelect
	langSelect      *limitSelect
	deviceSelect    *pointerSelect
	threadsSelect   *limitSelect
	speakersSelect  *pointerSelect
	expirySelect    *pointerSelect
	logRetainSelect *pointerSelect

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

	langCode := p.cfg.Language
	if langCode == "" {
		langCode = "auto"
	}
	langLabel := languageDisplayName(langCode)
	p.langSelect = newLimitSelect(allLanguageNames(), 300, p.onLangChanged)
	p.langSelect.Inner.Selected = langLabel
	p.langMirrors = append(p.langMirrors, p.langSelect)

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

	p.logRetainSelect = newPointerSelect(logRetentionOptions, func(string) {
		shared.SetLogRetentionDays(logRetentionLabelToDays(p.logRetainSelect.Selected))
		_ = p.writeConfig()
	})
	p.logRetainSelect.Selected = logRetentionDaysToLabel(p.cfg.LogRetentionDays)
	shared.SetLogRetentionDays(p.cfg.LogRetentionDays)

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
	viewLogBtn := newSecondaryButton("VIEW LOG", p.onViewLog)
	clearLogBtn := newSecondaryButton("CLEAR LOG", p.onClearLog)

	settingsGrid := container.NewGridWithColumns(2,
		newFormField("MODEL", p.modelSelect),
		newFormField("LANGUAGE", p.langSelect),
		newFormField("DEVICE", p.deviceSelect),
		newFormField("THREADS", p.threadsSelect),
		newFormField("SPEAKERS", p.speakersSelect),
		newFormField("CACHE EXPIRY (DAYS)", p.expirySelect),
		newFormField("LOG RETENTION", p.logRetainSelect),
	)

	toggleRow := container.NewGridWithColumns(2,
		wrapGhost(p.noDiarizeBtn),
		wrapGhost(p.debugBtn),
	)
	clearRow := container.NewGridWithColumns(2,
		wrapAction(clearCacheBtn),
		wrapAction(clearTranscriptsBtn),
	)
	logRow := container.NewGridWithColumns(2,
		wrapAction(viewLogBtn),
		wrapAction(clearLogBtn),
	)
	p.actionRow = container.NewVBox(
		toggleRow,
		vGap(spaceSM),
		clearRow,
		vGap(spaceSM),
		logRow,
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

func (p *settingsPanel) onViewLog() {
	tail := shared.ReadLogTail(256 * 1024)
	if tail == "" {
		tail = "(log is empty — transcribe something first)\n\nLog path:\n" + shared.LogFilePath()
	}
	preview.ShowText(preview.TextViewerOpts{
		Window: p.window,
		Title:  "wt.log",
		Body:   tail,
	})
}

func (p *settingsPanel) onClearLog() {
	showConfirm(p.window, "Clear log",
		"Erase the persistent log file? In-memory log is unaffected.",
		func() {
			if err := shared.ClearLog(); err != nil {
				showError(p.window, err)
				return
			}
			showDialog(dialogConfig{Parent: p.window, Title: "Log cleared", Body: widget.NewLabel(shared.LogFilePath())})
		})
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

	cfg.Model = displayNameToWhisperSize(p.modelSelect.Selected, p.cfg.Model)
	cfg.Language = p.Language()
	cfg.Device = p.deviceSelect.Selected
	cfg.Threads = p.Threads()
	cfg.Speakers = p.Speakers()
	cfg.NoDiarize = p.noDiarizeState
	cfg.CacheExpiryDays = p.cacheExpiryDays()
	cfg.LogRetentionDays = logRetentionLabelToDays(p.logRetainSelect.Selected)

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

	return languageCodeFromName(p.langSelect.Inner.Selected)
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

func (p *settingsPanel) refreshLanguageOptions() {
	if len(p.langMirrors) == 0 {
		return
	}
	mgr := models.NewManager()
	allowed := supportedLanguagesForActive(mgr)

	currentName := p.langSelect.Inner.Selected
	currentCode := languageCodeFromName(currentName)
	if currentCode == "" {
		currentCode = "auto"
	}
	codes, selectedCode := filterLanguageOptions(allLanguageCodes(), allowed, currentCode)
	opts := codesToNames(codes)
	selectedName := languageDisplayName(selectedCode)

	for _, m := range p.langMirrors {
		m.Inner.Options = opts
		m.Inner.Selected = selectedName

		if len(opts) <= 1 {
			m.Inner.Disable()
		} else {
			m.Inner.Enable()
		}
		m.Inner.Refresh()
	}
	if currentName != selectedName {
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

	s := newLimitSelect(p.langSelect.Inner.Options, 300, p.onLangChanged)
	s.Inner.Selected = p.langSelect.Inner.Selected
	if p.langSelect.Inner.Disabled() {
		s.Inner.Disable()
	}
	p.langMirrors = append(p.langMirrors, s)
	return s
}

func (p *settingsPanel) newSpeakersSelectMirror() *pointerSelect {
	s := newPointerSelect(speakerOptions, p.onSpeakersChanged)
	s.Selected = p.speakersSelect.Selected
	p.speakersMirrors = append(p.speakersMirrors, s)
	return s
}
