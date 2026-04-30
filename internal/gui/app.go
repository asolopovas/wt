//go:build !android

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"

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
	transcribe.attachLibrary(history)
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

	transcodeTab := buildTranscodeTab(transcribe)
	logTab := buildLogTab(transcribe)
	settingsTab := buildSettingsTab(settings, deviceInfo)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("TRANSCODE", theme.MediaRecordIcon(), transcodeTab),
		container.NewTabItemWithIcon("LOG", theme.DocumentIcon(), logTab),
		container.NewTabItemWithIcon("SETTINGS", theme.SettingsIcon(), settingsTab),
	)
	tabs.SetTabLocation(container.TabLocationBottom)

	setupTray(a, w, transcribe)

	w.SetContent(tabs)
	w.ShowAndRun()
	return nil
}

func buildTranscodeTab(tp *transcribePanel) fyne.CanvasObject {
	addBtn := newSecondaryButton("ADD FILES", tp.onBrowse)

	sidebar := buildSidebar(tp, addBtn)

	return container.New(newSidebarLayout(spaceLG), tp.container, sidebar)
}

func buildSidebar(tp *transcribePanel, addBtn *pointerButton) fyne.CanvasObject {
	optionsBlock := container.NewVBox(
		newSectionHeader("OPTIONS"),
		container.New(newCappedGrid(3, spaceMD, 0),
			newFormField("MODEL", tp.settings.modelSelect),
			newFormField("LANGUAGE", tp.settings.langSelect),
			newFormField("SPEAKERS", tp.settings.speakersSelect),
		),
	)

	actionsBlock := container.NewVBox(
		newSectionHeader("ACTIONS"),
		container.New(newCappedGrid(1, spaceLG, 36),
			wrapAction(addBtn),
		),
	)

	statusRow := container.NewBorder(nil, nil, tp.statusText, tp.timerText)
	statusBlock := container.NewVBox(
		newSectionHeader("STATUS"),
		tp.progress,
		statusRow,
	)

	content := container.NewVBox(
		optionsBlock,
		newSectionDivider(),
		actionsBlock,
		newSectionDivider(),
		statusBlock,
	)

	scroll := container.NewVScroll(content)

	return container.NewStack(newPanelBackground(), container.NewPadded(scroll))
}

func buildSettingsTab(sp *settingsPanel, deviceInfo string) fyne.CanvasObject {
	deviceHeader := newCaptionText("DEVICE")
	deviceHeader.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	deviceLabel := canvas.NewText(deviceInfo, colSecondary)
	deviceLabel.TextSize = textBody
	deviceLabel.TextStyle = fyne.TextStyle{Monospace: true}

	gap := canvas.NewRectangle(transparent)
	gap.SetMinSize(fyne.NewSize(0, spaceXL))

	return container.NewBorder(
		nil,
		container.NewVBox(gap, deviceHeader, deviceLabel),
		nil, nil,
		sp.container,
	)
}
