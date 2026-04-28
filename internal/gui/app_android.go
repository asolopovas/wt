//go:build android

package gui

import (
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
	settings.onCacheCleared = history.refresh

	if cacheGC(cfg.CacheExpiryDays) > 0 {
		history.rebuild()
	}

	deviceInfo := detectDevice()

	filesTab := transcribe.buildFilesTab()
	transcodeTab := buildTranscodeTabAndroid(transcribe, deviceInfo)
	settingsTab := buildSettingsTab(settings)

	tabs := container.NewAppTabs(
		container.NewTabItem("FILES", filesTab),
		container.NewTabItem("TRANSCODE", transcodeTab),
		container.NewTabItem("HISTORY", history.container),
		container.NewTabItem("SETTINGS", settingsTab),
	)
	tabs.SetTabLocation(container.TabLocationBottom)

	w.SetContent(tabs)
	w.ShowAndRun()
	return nil
}

func buildTranscodeTabAndroid(tp *transcribePanel, deviceInfo string) fyne.CanvasObject {
	deviceLabel := canvas.NewText(deviceInfo, colSecondary)
	deviceLabel.TextSize = 10
	deviceLabel.TextStyle = fyne.TextStyle{Monospace: true}

	tp.transcribeBtn.Importance = widget.HighImportance

	tp.exportBtn = newPointerButton("EXPORT", tp.onExport)
	tp.exportBtn.Importance = widget.LowImportance
	tp.openBtn = newPointerButton("OPEN", tp.onOpen)
	tp.openBtn.Importance = widget.LowImportance
	tp.previewBtn = newPointerButton("PREVIEW", tp.onPreview)
	tp.previewBtn.Importance = widget.LowImportance
	tp.previewBtn.Disable()
	tp.clearBtn.SetText("CLEAR LOG")
	tp.clearBtn.Importance = widget.LowImportance

	actionRow1 := container.NewGridWithColumns(2,
		borderedBtn(tp.clearBtn, colOutline),
		borderedBtn(tp.exportBtn, colOutline),
	)

	actionRow2 := container.NewGridWithColumns(2,
		borderedBtn(tp.openBtn, colOutline),
		borderedBtn(tp.previewBtn, colOutline),
	)

	dateAndTime := container.NewGridWithColumns(2, tp.dateEntry, tp.timeEntry)
	startTimeRow := settingsField("RECORDED AT", dateAndTime)

	bottomGap := canvas.NewRectangle(transparent)
	bottomGap.SetMinSize(fyne.NewSize(0, 6))

	bottomBar := container.NewVBox(
		tp.progress,
		tp.statusText,
		tp.statsLine,
		deviceLabel,
		startTimeRow,
		actionRow1,
		actionRow2,
		borderedBtn(tp.transcribeBtn, colPrimary),
		bottomGap,
	)

	return container.NewBorder(
		nil, bottomBar, nil, nil,
		tp.container,
	)
}

func buildSettingsTab(sp *settingsPanel) fyne.CanvasObject {
	settingsGrid := container.NewGridWithColumns(2,
		settingsField("MODEL", sp.modelSelect),
		settingsField("LANGUAGE", sp.langSelect),
		settingsField("DEVICE", sp.deviceSelect),
		settingsField("THREADS", sp.threadsSelect),
		settingsField("SPEAKERS", sp.speakersSelect),
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

	topSection := container.NewVBox(
		gap(12),
		header,
		gap(16),
		settingsGrid,
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
