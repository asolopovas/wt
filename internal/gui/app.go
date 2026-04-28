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

	transcodeTab := buildTranscodeTab(transcribe, deviceInfo)
	settingsTab := buildSettingsTab(settings)

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

func buildTranscodeTab(tp *transcribePanel, deviceInfo string) fyne.CanvasObject {
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
	tp.clearBtn.SetText("CLEAR")
	tp.clearBtn.Importance = widget.LowImportance

	actionRow := container.NewGridWithColumns(4,
		borderedBtn(tp.clearBtn, colOutline),
		borderedBtn(tp.exportBtn, colOutline),
		borderedBtn(tp.openBtn, colOutline),
		borderedBtn(tp.previewBtn, colOutline),
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
		tp.statusText,
		tp.statsLine,
		deviceLabel,
		optionsRow,
		startTimeRow,
		actionRow,
		tp.transcribeBtn,
		btnSpacer,
	)

	return container.NewBorder(
		nil, bottomBar, nil, nil,
		tp.container,
	)
}

func buildSettingsTab(sp *settingsPanel) fyne.CanvasObject {
	return sp.container
}
