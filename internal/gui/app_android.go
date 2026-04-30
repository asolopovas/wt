//go:build android

package gui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/gui/assets"
	"github.com/asolopovas/wt/internal/gui/cache"
	"github.com/asolopovas/wt/internal/gui/decor"
	"github.com/asolopovas/wt/internal/gui/platsvc"
	"github.com/asolopovas/wt/internal/gui/transcribe"
)

func Run(version string) error {
	cfg, _ := shared.Load()

	a := app.New()
	a.SetIcon(appIcon)
	a.Settings().SetTheme(&whisperTheme{})

	w := a.NewWindow("wt " + version)
	w.SetIcon(appIcon)

	settings := newSettingsPanel(cfg, w)
	tp := transcribe.New(w, settings)
	history := newHistoryPanel(w, tp)
	tp.History = history
	attachLibrary(tp, history)
	settings.onCacheCleared = history.Refresh

	if cache.GC(cfg.CacheExpiryDays) > 0 {
		history.rebuild()
	}
	go func() {
		if cache.BackfillDurations() > 0 {
			fyne.Do(history.Refresh)
		}
	}()

	deviceInfo := detectDevice()

	transcodeTab := buildTranscodeTabAndroid(tp, settings)
	logTab := transcribe.BuildLogTab(tp)
	settingsTab := buildSettingsTab(settings, deviceInfo)

	if missing := platsvc.MissingPermissions(); len(missing) > 0 {
		go func(p []string) {
			time.Sleep(600 * time.Millisecond)
			fyne.Do(func() { platsvc.RequestPermissions(p) })
		}(missing)
	}

	settingsTabItem := container.NewTabItem("SETTINGS", settingsTab)
	tabs := container.NewAppTabs(
		container.NewTabItem("TRANSCODE", transcodeTab),
		container.NewTabItem("LOG", logTab),
		settingsTabItem,
	)
	tabs.SetTabLocation(container.TabLocationBottom)
	tabs.OnSelected = func(t *container.TabItem) {
		if t == settingsTabItem && permsSection != nil {
			permsSection.refresh()
		}
	}

	w.SetContent(tabs)
	wireShareIntake(tp, tabs)
	w.ShowAndRun()
	return nil
}

func wireShareIntake(tp *transcribe.Panel, tabs *container.AppTabs) {
	go func() {
		for path := range platsvc.ShareIntakeChan() {
			path := path
			fyne.Do(func() {
				if tp.AddLocalFile(path) {
					tp.RebuildChips()
					tp.UpdateDropLabel()
					tp.AppendLog("Imported shared file: " + path)
					if tabs != nil {
						tabs.SelectIndex(0)
					}
				}
			})
		}
	}()
	platsvc.PollShareIntent()
	go func() {
		ticker := time.NewTicker(750 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			platsvc.PollShareIntent()
		}
	}()
}

func buildTranscodeTabAndroid(tp *transcribe.Panel, settings *settingsPanel) fyne.CanvasObject {
	tp.TranscribeBtn.Importance = widget.HighImportance

	addBtn := newSecondaryButton("ADD FILES", tp.OnBrowse)
	cancelBtn := decor.NewDangerButton("CANCEL", tp.OnCancel)

	var recBtn *pointerButton
	recBtn = newPointerButtonWithIcon("RECORD", assets.MicIcon, func() { tp.OnToggleRecord(recBtn) })
	recBtn.Importance = widget.HighImportance

	settingsRow := container.NewGridWithColumns(3,
		newFormField("MODEL", settings.modelSelect),
		newFormField("LANGUAGE", settings.langSelect),
		newFormField("SPEAKERS", settings.speakersSelect),
	)

	actionRow := container.NewGridWithColumns(3,
		wrapAction(recBtn),
		wrapAction(addBtn),
		wrapAction(cancelBtn),
	)

	statsRight := container.NewHBox(tp.StatsLine, tp.TimerText)
	bottomBar := container.NewVBox(
		tp.Progress,
		container.NewBorder(nil, nil, tp.StatusText, statsRight),
		settingsRow,
		actionRow,
		vGap(spaceMD),
	)

	return container.NewBorder(
		nil, bottomBar, nil, nil,
		tp.Container,
	)
}

var permsSection *permissionsSection

func inlineField(label string, w fyne.CanvasObject) fyne.CanvasObject {
	lbl := canvas.NewText(label, decor.TextMuted)
	lbl.TextSize = textCaption
	lbl.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	left := container.New(&fixedWidthLayout{width: 90}, lbl)
	return container.NewBorder(nil, nil, left, nil, w)
}

func buildSettingsTab(sp *settingsPanel, deviceInfo string) fyne.CanvasObject {
	settingsGrid := container.NewVBox(
		inlineField("DEVICE", sp.deviceSelect),
		inlineField("THREADS", sp.threadsSelect),
		inlineField("CACHE EXPIRY", sp.expirySelect),
	)

	header := decor.NewPanelHeader(newCaptionText("SETTINGS"))

	_ = deviceInfo
	statsBlock := container.NewVBox()
	for _, st := range deviceStats() {
		val := widget.NewLabel(st.Value)
		val.TextStyle = fyne.TextStyle{Monospace: true}
		val.Wrapping = fyne.TextWrapWord
		statsBlock.Add(inlineField(st.Label, val))
	}

	if permsSection == nil {
		permsSection = newPermissionsSection()
	}

	bodySection := container.NewVBox(
		vGap(spaceMD),
		statsBlock,
		vGap(spaceXL),
		settingsGrid,
		vGap(spaceXXL),
		permsSection.container,
	)

	toggleRow := container.NewGridWithColumns(2,
		wrapGhost(sp.noDiarizeBtn),
		wrapGhost(sp.debugBtn),
	)

	clearCacheBtn := newSecondaryButton("CACHE", sp.onClearCache)
	clearTranscriptsBtn := newSecondaryButton("TEXT", sp.onClearTranscripts)

	cacheRow := container.NewGridWithColumns(3,
		wrapAction(clearCacheBtn),
		wrapAction(clearTranscriptsBtn),
		wrapAction(sp.saveBtn),
	)

	bottomSection := container.NewVBox(
		vGap(spaceMD),
		toggleRow,
		vGap(spaceMD),
		cacheRow,
		vGap(spaceMD),
	)

	pad := func(o fyne.CanvasObject) fyne.CanvasObject {
		return container.New(&insetLayout{padX: spaceXL, padY: 0}, o)
	}

	return container.NewBorder(
		header, pad(bottomSection), nil, nil,
		container.NewVScroll(pad(bodySection)),
	)
}
