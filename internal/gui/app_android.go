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
)

func Run(version string) error {
	cfg, _ := shared.Load()

	a := app.New()
	a.SetIcon(appIcon)
	a.Settings().SetTheme(&whisperTheme{})

	w := a.NewWindow("wt " + version)
	w.SetIcon(appIcon)

	settings := newSettingsPanel(cfg, w)
	transcribe := newTranscribePanel(w, settings)
	history := newHistoryPanel(w, transcribe)
	transcribe.history = history
	transcribe.attachLibrary(history)
	if history.headerRight != nil {
		history.headerRight.Objects = []fyne.CanvasObject{transcribe.statsLine, transcribe.timerText}
		history.headerRight.Refresh()
	}
	settings.onCacheCleared = history.refresh

	if cacheGC(cfg.CacheExpiryDays) > 0 {
		history.rebuild()
	}

	deviceInfo := detectDevice()

	transcodeTab := buildTranscodeTabAndroid(transcribe)
	logTab := buildLogTab(transcribe)
	settingsTab := buildSettingsTab(settings, deviceInfo)

	if missing := missingPermissions(); len(missing) > 0 {
		go func(p []string) {
			time.Sleep(600 * time.Millisecond)
			fyne.Do(func() { requestPermissions(p) })
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
	wireShareIntake(transcribe, tabs)
	w.ShowAndRun()
	return nil
}

func wireShareIntake(tp *transcribePanel, tabs *container.AppTabs) {
	go func() {
		for path := range shareIntakeChan() {
			path := path
			fyne.Do(func() {
				if tp.addLocalFile(path) {
					tp.rebuildChips()
					tp.updateDropLabel()
					tp.appendLog("Imported shared file: " + path)
					if tabs != nil {
						tabs.SelectIndex(0)
					}
				}
			})
		}
	}()
	pollShareIntent()
	go func() {
		ticker := time.NewTicker(750 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			pollShareIntent()
		}
	}()
}

func buildTranscodeTabAndroid(tp *transcribePanel) fyne.CanvasObject {
	tp.transcribeBtn.Importance = widget.HighImportance

	addBtn := newPointerButton("ADD FILES", tp.onBrowse)
	addBtn.Importance = widget.LowImportance

	cancelBtn := newPointerButton("CANCEL", tp.onCancel)
	cancelBtn.Importance = widget.DangerImportance

	var recBtn *pointerButton
	recBtn = newPointerButtonWithIcon("RECORD", micIconResource, func() { tp.onToggleRecord(recBtn) })
	recBtn.Importance = widget.HighImportance

	settingsRow := container.NewGridWithColumns(3,
		settingsField("MODEL", tp.settings.modelSelect),
		settingsField("LANGUAGE", tp.settings.langSelect),
		settingsField("SPEAKERS", tp.settings.speakersSelect),
	)

	actionRow := container.NewGridWithColumns(3,
		borderedBtn(recBtn, colPrimary),
		borderedBtn(addBtn, colOutline),
		borderedBtn(cancelBtn, colError),
	)

	bottomGap := canvas.NewRectangle(transparent)
	bottomGap.SetMinSize(fyne.NewSize(0, 6))

	bottomBar := container.NewVBox(
		tp.progress,
		container.NewBorder(nil, nil, tp.statusText, nil),
		settingsRow,
		actionRow,
		bottomGap,
	)

	return container.NewBorder(
		nil, bottomBar, nil, nil,
		tp.container,
	)
}

var permsSection *permissionsSection

func buildSettingsTab(sp *settingsPanel, deviceInfo string) fyne.CanvasObject {
	settingsGrid := container.NewGridWithColumns(2,
		settingsField("DEVICE", sp.deviceSelect),
		settingsField("THREADS", sp.threadsSelect),
		settingsField("EXPIRY (DAYS)", sp.expirySelect),
	)

	gap := func(h float32) fyne.CanvasObject {
		r := canvas.NewRectangle(transparent)
		r.SetMinSize(fyne.NewSize(0, h))
		return r
	}

	header := canvas.NewText("SETTINGS", colMuted)
	header.TextSize = 12
	header.TextStyle = fyne.TextStyle{Bold: true}
	header.Alignment = fyne.TextAlignCenter

	deviceHeader := canvas.NewText("DEVICE", colMuted)
	deviceHeader.TextSize = 10
	deviceHeader.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	deviceLabel := widget.NewLabel(deviceInfo)
	deviceLabel.TextStyle = fyne.TextStyle{Monospace: true}
	deviceLabel.Wrapping = fyne.TextWrapWord

	if permsSection == nil {
		permsSection = newPermissionsSection()
	}

	topSection := container.NewVBox(
		gap(12),
		header,
		gap(16),
		settingsGrid,
		gap(12),
		deviceHeader,
		deviceLabel,
		gap(16),
		permsSection.container,
	)

	toggleRow := container.NewGridWithColumns(2,
		borderedBtn(sp.noDiarizeBtn, colPrimaryGhost),
		borderedBtn(sp.debugBtn, colOutline),
	)

	clearCacheBtn := newPointerButton("CLEAR CACHE", sp.onClearCache)
	clearCacheBtn.Importance = widget.LowImportance

	clearTranscriptsBtn := newPointerButton("CLEAR TEXT", sp.onClearTranscripts)
	clearTranscriptsBtn.Importance = widget.LowImportance

	cacheRow := container.NewGridWithColumns(2,
		borderedBtn(clearCacheBtn, colOutline),
		borderedBtn(clearTranscriptsBtn, colOutline),
	)

	actionRow := container.NewGridWithColumns(1,
		borderedBtn(sp.saveBtn, colPrimary),
	)

	bottomGap := canvas.NewRectangle(transparent)
	bottomGap.SetMinSize(fyne.NewSize(0, 6))

	bottomSection := container.NewVBox(
		toggleRow,
		cacheRow,
		actionRow,
		bottomGap,
	)

	return container.NewBorder(
		nil, bottomSection, nil, nil,
		container.NewVScroll(topSection),
	)
}
