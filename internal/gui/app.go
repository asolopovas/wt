//go:build !android

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/appinfo"
	"github.com/asolopovas/wt/internal/gui/cache"
	"github.com/asolopovas/wt/internal/gui/decor"
	"github.com/asolopovas/wt/internal/gui/transcribe"
)

func Run(version, buildDate string) error {
	cfg, _ := shared.Load()

	a := app.New()
	a.SetIcon(appIcon)
	a.Settings().SetTheme(&whisperTheme{})

	w := a.NewWindow(appinfo.Name + " " + version)
	w.SetIcon(appIcon)
	w.Resize(fyne.NewSize(1040, 720))

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

	transcodeTab := buildTranscodeTab(tp, settings)
	logTab := transcribe.BuildLogTab(tp)
	settingsTab := buildSettingsTab(settings, deviceInfo, versionLabel(version, buildDate))

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("TRANSCODE", theme.MediaRecordIcon(), transcodeTab),
		container.NewTabItemWithIcon("LOG", theme.DocumentIcon(), logTab),
		container.NewTabItemWithIcon("SETTINGS", theme.SettingsIcon(), settingsTab),
	)
	tabs.SetTabLocation(container.TabLocationBottom)

	transcribe.SetupTray(a, w, tp, appIcon)

	w.SetContent(tabs)
	w.ShowAndRun()
	return nil
}

func buildTranscodeTab(tp *transcribe.Panel, settings *settingsPanel) fyne.CanvasObject {
	addBtn := newSecondaryButton("ADD FILES", tp.OnBrowse)

	sidebar := buildSidebar(tp, settings, addBtn)

	return container.New(newSidebarLayout(spaceLG), tp.Container, sidebar)
}

func buildSidebar(tp *transcribe.Panel, settings *settingsPanel, addBtn *pointerButton) fyne.CanvasObject {
	optionsBlock := container.NewVBox(
		newSectionHeader("OPTIONS"),
		container.New(newCappedGrid(3, spaceMD, 0),
			newFormField("MODEL", settings.modelSelect),
			newFormField("LANGUAGE", settings.langSelect),
			newFormField("SPEAKERS", settings.speakersSelect),
		),
	)

	actionsBlock := container.NewVBox(
		newSectionHeader("ACTIONS"),
		container.New(newCappedGrid(1, spaceLG, 36),
			wrapAction(addBtn),
		),
	)

	statusRow := container.NewBorder(nil, nil, tp.StatusText, tp.TimerText)
	statusBlock := container.NewVBox(
		newSectionHeader("STATUS"),
		tp.Progress,
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

func buildSettingsTab(sp *settingsPanel, deviceInfo, version string) fyne.CanvasObject {
	deviceHeader := newCaptionText("DEVICE")
	deviceHeader.TextStyle = monoBoldStyle
	deviceLabel := canvas.NewText(deviceInfo, colSecondary)
	deviceLabel.TextSize = textBody
	deviceLabel.TextStyle = fyne.TextStyle{Monospace: true}

	versionLabel := canvas.NewText(version, colSecondary)
	versionLabel.TextSize = textCaption
	versionLabel.TextStyle = monoBoldStyle
	header := decor.NewPanelHeader(newCaptionText("SETTINGS"), versionLabel)

	return container.NewBorder(
		header,
		container.NewVBox(vGap(spaceXL), deviceHeader, deviceLabel),
		nil, nil,
		container.NewVScroll(sp.container),
	)
}
