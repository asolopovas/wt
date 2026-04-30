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
	go func() {
		if cacheBackfillDurations() > 0 {
			fyne.Do(history.refresh)
		}
	}()

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

	addBtn := newSecondaryButton("ADD FILES", tp.onBrowse)
	cancelBtn := newDangerButton("CANCEL", tp.onCancel)

	var recBtn *pointerButton
	recBtn = newPointerButtonWithIcon("RECORD", micIconResource, func() { tp.onToggleRecord(recBtn) })
	recBtn.Importance = widget.HighImportance

	settingsRow := container.NewGridWithColumns(3,
		newFormField("MODEL", tp.settings.modelSelect),
		newFormField("LANGUAGE", tp.settings.langSelect),
		newFormField("SPEAKERS", tp.settings.speakersSelect),
	)

	actionRow := container.NewGridWithColumns(3,
		wrapAction(recBtn),
		wrapAction(addBtn),
		wrapAction(cancelBtn),
	)

	bottomGap := canvas.NewRectangle(transparent)
	bottomGap.SetMinSize(fyne.NewSize(0, spaceMD))

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
		newFormField("DEVICE", sp.deviceSelect),
		newFormField("THREADS", sp.threadsSelect),
		newFormField("EXPIRY (DAYS)", sp.expirySelect),
	)

	gap := func(h float32) fyne.CanvasObject {
		r := canvas.NewRectangle(transparent)
		r.SetMinSize(fyne.NewSize(0, h))
		return r
	}

	header := canvas.NewText("SETTINGS", colMuted)
	header.TextSize = textLabel
	header.TextStyle = fyne.TextStyle{Bold: true}
	header.Alignment = fyne.TextAlignCenter

	deviceHeader := newCaptionText("DEVICE")
	deviceHeader.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	deviceLabel := widget.NewLabel(deviceInfo)
	deviceLabel.TextStyle = fyne.TextStyle{Monospace: true}
	deviceLabel.Wrapping = fyne.TextWrapWord

	if permsSection == nil {
		permsSection = newPermissionsSection()
	}

	topSection := container.NewVBox(
		gap(spaceXL),
		header,
		gap(spaceXXL),
		settingsGrid,
		gap(spaceXL),
		deviceHeader,
		deviceLabel,
		gap(spaceXXL),
		permsSection.container,
	)

	toggleRow := container.NewGridWithColumns(2,
		wrapGhost(sp.noDiarizeBtn),
		wrapGhost(sp.debugBtn),
	)

	clearCacheBtn := newSecondaryButton("CLEAR CACHE", sp.onClearCache)
	clearTranscriptsBtn := newSecondaryButton("CLEAR TEXT", sp.onClearTranscripts)

	cacheRow := container.NewGridWithColumns(2,
		wrapAction(clearCacheBtn),
		wrapAction(clearTranscriptsBtn),
	)

	actionRow := container.NewGridWithColumns(1,
		wrapAction(sp.saveBtn),
	)

	bottomGap := canvas.NewRectangle(transparent)
	bottomGap.SetMinSize(fyne.NewSize(0, spaceMD))

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
