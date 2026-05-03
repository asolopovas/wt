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
	"github.com/asolopovas/wt/internal/gui/transcribe"
	"github.com/asolopovas/wt/internal/transcriber/cache"
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
	dock := newPlayerDock(w, tp)
	history := newHistoryPanel(w, tp, dock)
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
	settingsTab := buildSettingsTab(settings, deviceInfo, versionLabel(version, buildDate))

	// LOG tab / workspace column was removed — the live log now lives
	// solely on disk (<MediaDir>/wt.log) and is reachable via Settings
	// → VIEW LOG. Two top-level tabs are enough.
	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("TRANSCODE", theme.MediaRecordIcon(), transcodeTab),
		container.NewTabItemWithIcon("SETTINGS", theme.SettingsIcon(), settingsTab),
	)
	tabs.SetTabLocation(container.TabLocationBottom)

	transcribe.SetupTray(a, w, tp, appIcon)

	rebuildContent := func() {
		w.SetContent(container.NewBorder(nil, dock.Container(), nil, nil, tabs))
	}
	dock.onVisibilityChange = rebuildContent
	rebuildContent()
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
		container.New(newCappedGrid(2, spaceMD, 0),
			newFormField("MODEL", settings.newModelSelectMirror()),
			newFormField("DIARIZER", settings.newDiarizerSelectMirror()),
			newFormField("LANGUAGE", settings.newLangSelectMirror()),
			newFormField("SPEAKERS", settings.newSpeakersSelectMirror()),
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
	deviceLabel := canvas.NewText(deviceInfo, colSecondary)
	deviceLabel.TextSize = textBody
	deviceLabel.TextStyle = fyne.TextStyle{Monospace: true}

	versionText := canvas.NewText(version, colSecondary)
	versionText.TextSize = textCaption
	versionText.TextStyle = monoBoldStyle

	optionsBlock := container.NewVBox(
		newSectionHeader("OPTIONS"),
		sp.settingsGrid,
	)
	modelsBlock := sp.models.container
	deviceBlock := container.NewVBox(
		newSectionHeader("DEVICE"),
		deviceLabel,
		vGap(spaceXS),
		versionText,
	)
	actionsBlock := container.NewVBox(
		newSectionHeader("ACTIONS"),
		sp.actionRow,
	)

	content := container.NewVBox(
		optionsBlock,
		newSectionDivider(),
		modelsBlock,
		newSectionDivider(),
		deviceBlock,
		newSectionDivider(),
		actionsBlock,
	)

	scroll := container.NewVScroll(content)
	return container.NewStack(newPanelBackground(), container.NewPadded(scroll))
}
