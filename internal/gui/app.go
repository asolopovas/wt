//go:build !android

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
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
	w.Resize(fyne.NewSize(780, 720))

	settings := newSettingsPanel(cfg, w)
	transcribe := newTranscribePanel(w, settings)
	history := newHistoryPanel(w, transcribe)
	transcribe.history = history
	settings.onCacheCleared = history.refresh

	if cacheGC(cfg.CacheExpiryDays) > 0 {
		history.rebuild()
	}

	deviceInfo := detectDevice()

	transcodeTab := buildTranscodeTab(transcribe)
	settingsTab := buildSettingsTab(settings, deviceInfo)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("TRANSCODE", theme.MediaRecordIcon(), transcodeTab),
		container.NewTabItemWithIcon("HISTORY", theme.ContentRedoIcon(), history.container),
		container.NewTabItemWithIcon("SETTINGS", theme.SettingsIcon(), settingsTab),
	)
	tabs.SetTabLocation(container.TabLocationBottom)

	w.SetContent(tabs)
	w.ShowAndRun()
	return nil
}

func buildTranscodeTab(tp *transcribePanel) fyne.CanvasObject {
	tp.transcribeBtn.Importance = widget.HighImportance

	tp.exportBtn = newPointerButton("EXPORT", tp.onExport)
	tp.exportBtn.Importance = widget.LowImportance
	tp.previewBtn = newPointerButton("PREVIEW", tp.onPreview)
	tp.previewBtn.Importance = widget.LowImportance
	tp.previewBtn.Disable()

	actionRow := container.NewGridWithColumns(3,
		borderedBtn(tp.previewBtn, colOutline),
		borderedBtn(tp.exportBtn, colOutline),
		borderedBtn(tp.transcribeBtn, colPrimary),
	)

	optionsRow := container.NewGridWithColumns(3,
		settingsField("MODEL", tp.settings.modelSelect),
		settingsField("LANGUAGE", tp.settings.langSelect),
		settingsField("SPEAKERS", tp.settings.speakersSelect),
	)

	nowBtn := newPointerButton("NOW", tp.onStartTimeBothNow)
	nowBtn.Importance = widget.LowImportance
	dateAndTime := container.NewGridWithColumns(3,
		borderedBtn(tp.dateBtn, colOutline),
		borderedBtn(tp.timeBtn, colOutline),
		borderedBtn(nowBtn, colOutline),
	)
	startTimeRow := settingsField("RECORDED AT", dateAndTime)

	btnSpacer := canvas.NewRectangle(transparent)
	btnSpacer.SetMinSize(fyne.NewSize(0, 4))

	bottomBar := container.NewVBox(
		tp.progress,
		container.NewBorder(nil, nil, tp.statusText, tp.timerText),
		optionsRow,
		startTimeRow,
		actionRow,
		btnSpacer,
	)

	return container.NewBorder(
		nil, bottomBar, nil, nil,
		tp.container,
	)
}

func buildSettingsTab(sp *settingsPanel, deviceInfo string) fyne.CanvasObject {
	deviceHeader := canvas.NewText("DEVICE", colMuted)
	deviceHeader.TextSize = 10
	deviceHeader.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	deviceLabel := canvas.NewText(deviceInfo, colSecondary)
	deviceLabel.TextSize = 11
	deviceLabel.TextStyle = fyne.TextStyle{Monospace: true}

	gap := canvas.NewRectangle(transparent)
	gap.SetMinSize(fyne.NewSize(0, 12))

	return container.NewBorder(
		nil,
		container.NewVBox(gap, deviceHeader, deviceLabel),
		nil, nil,
		sp.container,
	)
}
