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
	w.Resize(fyne.NewSize(1040, 720))

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
		container.NewTabItemWithIcon("SETTINGS", theme.SettingsIcon(), settingsTab),
	)
	tabs.SetTabLocation(container.TabLocationBottom)

	w.SetContent(tabs)
	w.ShowAndRun()
	return nil
}

func buildTranscodeTab(tp *transcribePanel) fyne.CanvasObject {
	tp.transcribeBtn.Importance = widget.HighImportance

	libraryBtn := newPointerButton("LIBRARY", tp.openLibrary)
	libraryBtn.Importance = widget.LowImportance

	sidebar := buildSidebar(tp, libraryBtn)

	return container.New(newSidebarLayout(8), tp.container, sidebar)
}

func buildSidebar(tp *transcribePanel, libraryBtn *pointerButton) fyne.CanvasObject {
	bg := canvas.NewRectangle(colSurfLowest)
	bg.StrokeColor = colGhostBorder
	bg.StrokeWidth = 1

	optionsBlock := container.NewVBox(
		sidebarHeader("OPTIONS"),
		container.New(newCappedGrid(3, 6, 0),
			settingsField("MODEL", tp.settings.modelSelect),
			settingsField("LANGUAGE", tp.settings.langSelect),
			settingsField("SPEAKERS", tp.settings.speakersSelect),
		),
	)

	actionsBlock := container.NewVBox(
		sidebarHeader("ACTIONS"),
		container.New(newCappedGrid(2, 8, 36),
			borderedBtn(libraryBtn, colDialogBorder),
			borderedBtn(tp.transcribeBtn, colDialogBorder),
		),
	)

	logBlock := container.NewVBox(
		sidebarHeader("LOG"),
		container.New(newCappedGrid(3, 6, 36),
			borderedBtn(tp.autoBtn, colDialogBorder),
			borderedBtn(tp.copyLogBtn, colDialogBorder),
			borderedBtn(tp.clearLogBtn, colDialogBorder),
		),
	)

	statusRow := container.NewBorder(nil, nil, tp.statusText, tp.timerText)
	statusBlock := container.NewVBox(
		sidebarHeader("STATUS"),
		tp.progress,
		statusRow,
	)

	content := container.NewVBox(
		optionsBlock,
		sidebarDivider(),
		actionsBlock,
		sidebarDivider(),
		logBlock,
		sidebarDivider(),
		statusBlock,
	)

	scroll := container.NewVScroll(content)

	return container.NewStack(bg, container.NewPadded(scroll))
}

func sidebarHeader(label string) fyne.CanvasObject {
	t := canvas.NewText(label, colMuted)
	t.TextSize = 10
	t.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	gap := canvas.NewRectangle(transparent)
	gap.SetMinSize(fyne.NewSize(0, 2))
	return container.NewVBox(t, gap)
}

func sidebarDivider() fyne.CanvasObject {
	line := canvas.NewRectangle(colGhostBorder)
	line.SetMinSize(fyne.NewSize(0, 1))
	pad := canvas.NewRectangle(transparent)
	pad.SetMinSize(fyne.NewSize(0, 6))
	return container.NewVBox(pad, line, pad)
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
