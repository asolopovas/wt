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
	settings.onCacheCleared = history.refresh

	if cacheGC(cfg.CacheExpiryDays) > 0 {
		history.rebuild()
	}

	deviceInfo := detectDevice()

	transcodeTab := buildTranscodeTabAndroid(transcribe)
	settingsTab := buildSettingsTab(settings, deviceInfo)

	tabs := container.NewAppTabs(
		container.NewTabItem("TRANSCODE", transcodeTab),
		container.NewTabItem("SETTINGS", settingsTab),
	)
	tabs.SetTabLocation(container.TabLocationBottom)

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

	settingsRow := container.NewGridWithColumns(3,
		settingsField("MODEL", tp.settings.modelSelect),
		settingsField("LANGUAGE", tp.settings.langSelect),
		settingsField("SPEAKERS", tp.settings.speakersSelect),
	)

	actionRow := container.NewGridWithColumns(1,
		borderedBtn(addBtn, colOutline),
	)

	bottomGap := canvas.NewRectangle(transparent)
	bottomGap.SetMinSize(fyne.NewSize(0, 6))

	bottomBar := container.NewVBox(
		tp.progress,
		container.NewBorder(nil, nil, tp.statusText, tp.timerText),
		settingsRow,
		actionRow,
		bottomGap,
	)

	return container.NewBorder(
		nil, bottomBar, nil, nil,
		tp.container,
	)
}

func buildSettingsTab(sp *settingsPanel, deviceInfo string) fyne.CanvasObject {
	settingsGrid := container.NewGridWithColumns(2,
		settingsField("DEVICE", sp.deviceSelect),
		settingsField("THREADS", sp.threadsSelect),
		settingsField("CACHE EXPIRY (DAYS)", sp.expirySelect),
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
	deviceLabel := canvas.NewText(deviceInfo, colSecondary)
	deviceLabel.TextSize = 11
	deviceLabel.TextStyle = fyne.TextStyle{Monospace: true}

	topSection := container.NewVBox(
		gap(12),
		header,
		gap(16),
		settingsGrid,
		gap(12),
		deviceHeader,
		deviceLabel,
	)

	toggleRow := container.NewGridWithColumns(2,
		borderedBtn(sp.noDiarizeBtn, colPrimaryGhost),
		borderedBtn(sp.debugBtn, colOutline),
	)

	clearCacheBtn := newPointerButton("CLEAR CACHE", sp.onClearCache)
	clearCacheBtn.Importance = widget.LowImportance

	clearTranscriptsBtn := newPointerButton("CLEAR TRANSCRIPTS", sp.onClearTranscripts)
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
		topSection, bottomSection, nil, nil,
	)
}
