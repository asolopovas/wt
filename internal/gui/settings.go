package gui

import (
	"fmt"
	"runtime"
	"slices"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	shared "github.com/asolopovas/wt/internal"
)

var languages = []string{
	"auto", "en", "zh", "de", "es", "ru", "ko", "fr", "ja", "pt",
	"tr", "pl", "ca", "nl", "ar", "sv", "it", "id", "hi", "fi",
	"vi", "he", "uk", "el", "ms", "cs", "ro", "da", "hu", "ta",
	"no", "th", "ur", "hr", "bg", "lt", "la", "mi", "ml", "cy",
}

var speakerOptions = []string{
	"auto", "2", "3", "4", "5", "6", "7", "8", "9", "10",
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
	noDiarizeBtn   *pointerButton
	noDiarizeState bool
	debugBtn       *pointerButton
	debugState     bool
	saveBtn        *pointerButton

	onCacheCleared func()

	container fyne.CanvasObject
}

func newSettingsPanel(cfg shared.Config, window fyne.Window) *settingsPanel {
	p := &settingsPanel{cfg: cfg, window: window, noDiarizeState: cfg.NoDiarize}
	p.build()
	return p
}

func (p *settingsPanel) build() {
	p.modelSelect = newPointerSelect(validModels, nil)
	p.modelSelect.Selected = p.cfg.Model
	if !slices.Contains(validModels, p.modelSelect.Selected) {
		p.modelSelect.Selected = validModels[0]
	}

	langLabel := "auto"
	if p.cfg.Language != "" {
		langLabel = p.cfg.Language
	}
	p.langSelect = newLimitSelect(languages, 300, nil)
	p.langSelect.inner.Selected = langLabel

	p.deviceSelect = newPointerSelect([]string{"auto", "cuda", "cpu"}, nil)
	p.deviceSelect.Selected = p.cfg.Device

	maxThreads := runtime.NumCPU()
	threadOpts := make([]string, maxThreads)
	for i := range maxThreads {
		threadOpts[i] = strconv.Itoa(i + 1)
	}
	p.threadsSelect = newLimitSelect(threadOpts, 300, nil)
	p.threadsSelect.inner.Selected = strconv.Itoa(p.cfg.Threads)

	p.speakersSelect = newPointerSelect(speakerOptions, nil)
	spkSel := "auto"
	if p.cfg.Speakers > 0 {
		spkSel = strconv.Itoa(p.cfg.Speakers)
	}
	p.speakersSelect.Selected = spkSel

	p.expirySelect = newPointerSelect(cacheExpiryOptions, nil)
	if p.cfg.CacheExpiryDays <= 0 {
		p.expirySelect.Selected = "never"
	} else {
		p.expirySelect.Selected = strconv.Itoa(p.cfg.CacheExpiryDays)
	}

	p.noDiarizeBtn = newPointerButton("", p.onToggleDiarize)
	p.updateDiarizeLabel()

	p.debugBtn = newPointerButton("", p.onToggleDebug)
	p.updateDebugLabel()

	p.saveBtn = newPointerButton("SAVE CONFIG", p.onSave)
	p.saveBtn.Importance = widget.HighImportance

	clearCacheBtn := newPointerButton("CLEAR CACHE", p.onClearCache)
	clearCacheBtn.Importance = widget.LowImportance

	clearTranscriptsBtn := newPointerButton("CLEAR TRANSCRIPTS", p.onClearTranscripts)
	clearTranscriptsBtn.Importance = widget.LowImportance

	settingsGrid := container.NewGridWithColumns(2,
		settingsField("MODEL", p.modelSelect),
		settingsField("LANGUAGE", p.langSelect),
		settingsField("DEVICE", p.deviceSelect),
		settingsField("THREADS", p.threadsSelect),
		settingsField("SPEAKERS", p.speakersSelect),
		settingsField("CACHE EXPIRY (DAYS)", p.expirySelect),
	)

	toggleRow := container.NewGridWithColumns(2,
		borderedBtn(p.noDiarizeBtn, colPrimaryGhost),
		borderedBtn(p.debugBtn, colOutline),
	)

	cacheRow := container.NewGridWithColumns(2,
		borderedBtn(clearCacheBtn, colOutline),
		borderedBtn(clearTranscriptsBtn, colOutline),
	)

	saveRow := borderedBtn(p.saveBtn, colPrimary)

	p.container = container.NewVBox(
		layout.NewSpacer(),
		settingsGrid,
		container.NewGridWrap(fyne.NewSize(0, 8)),
		toggleRow,
		container.NewGridWrap(fyne.NewSize(0, 4)),
		cacheRow,
		container.NewGridWrap(fyne.NewSize(0, 8)),
		saveRow,
		layout.NewSpacer(),
	)
}

func settingsField(label string, w fyne.CanvasObject) *fyne.Container {
	lbl := canvas.NewText(label, colMuted)
	lbl.TextSize = 10
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewVBox(lbl, w)
}

func (p *settingsPanel) onToggleDiarize() {
	p.noDiarizeState = !p.noDiarizeState
	p.updateDiarizeLabel()
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

func (p *settingsPanel) debug() bool {
	return p.debugState
}

func (p *settingsPanel) onClearCache() {
	clearCache(p.window, func(_ string) {})
	dialog.ShowInformation("Cache", "Audio cache cleared.", p.window)
}

func (p *settingsPanel) onClearTranscripts() {
	dialog.ShowConfirm("Clear transcripts",
		"Remove all cached transcripts? This cannot be undone.",
		func(ok bool) {
			if !ok {
				return
			}
			if err := cacheClear(); err != nil {
				dialog.ShowError(err, p.window)
				return
			}
			if p.onCacheCleared != nil {
				p.onCacheCleared()
			}
			dialog.ShowInformation("Transcripts", "Transcript cache cleared.", p.window)
		}, p.window)
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
	cfg, err := shared.Load()
	if err != nil {
		dialog.ShowError(fmt.Errorf("loading config: %w", err), p.window)
		return
	}

	cfg.Model = p.modelSelect.Selected
	cfg.Language = p.language()
	cfg.Device = p.deviceSelect.Selected
	cfg.Threads = p.threads()
	cfg.Speakers = p.speakers()
	cfg.NoDiarize = p.noDiarizeState
	cfg.CacheExpiryDays = p.cacheExpiryDays()

	if err := shared.Save(cfg); err != nil {
		dialog.ShowError(fmt.Errorf("saving settings: %w", err), p.window)
		return
	}
	dialog.ShowInformation("Settings", "Settings saved.", p.window)
}

func (p *settingsPanel) modelSize() string {
	return p.modelSelect.Selected
}

func (p *settingsPanel) language() string {
	lang := p.langSelect.inner.Selected
	if lang == "auto" {
		return ""
	}
	return lang
}

func (p *settingsPanel) device() string {
	return p.deviceSelect.Selected
}

func (p *settingsPanel) threads() int {
	n, err := strconv.Atoi(p.threadsSelect.inner.Selected)
	if err != nil || n < 1 {
		return runtime.NumCPU()
	}
	return n
}

func (p *settingsPanel) speakers() int {
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

func (p *settingsPanel) noDiarize() bool {
	return p.noDiarizeState
}
