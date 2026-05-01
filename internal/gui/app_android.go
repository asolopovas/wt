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
	"github.com/asolopovas/wt/internal/appinfo"
	"github.com/asolopovas/wt/internal/gui/assets"
	"github.com/asolopovas/wt/internal/gui/cache"
	"github.com/asolopovas/wt/internal/gui/decor"
	"github.com/asolopovas/wt/internal/gui/platsvc"
	"github.com/asolopovas/wt/internal/gui/transcribe"
)

func Run(version, buildDate string) error {
	cfg, _ := shared.Load()

	a := app.New()
	a.SetIcon(appIcon)
	a.Settings().SetTheme(&whisperTheme{})

	w := a.NewWindow(appinfo.Name + " " + version)
	w.SetIcon(appIcon)

	settings := newSettingsPanel(cfg, w)
	tp := transcribe.New(w, settings)
	history := newHistoryPanel(w, tp)
	tp.History = history
	attachLibrary(tp, history)
	if history.headerRight != nil {
		history.headerRight.Objects = []fyne.CanvasObject{tp.StatsLine, tp.TimerText}
		history.headerRight.Refresh()
	}
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

	transcodeTab := buildTranscodeTabAndroid(tp, settings)
	logTab := transcribe.BuildLogTab(tp)
	settingsTab := buildSettingsTab(settings, deviceInfo, versionLabel(version, buildDate))

	if missing := platsvc.MissingPermissions(); len(missing) > 0 {
		go func(p []string) {
			time.Sleep(600 * time.Millisecond)
			fyne.Do(func() { platsvc.RequestPermissions(p) })
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
	wireShareIntake(tp, tabs)
	w.ShowAndRun()
	return nil
}

func wireShareIntake(tp *transcribe.Panel, tabs *container.AppTabs) {
	go func() {
		for path := range platsvc.ShareIntakeChan() {
			path := path
			fyne.Do(func() {
				if tp.AddLocalFile(path) {
					tp.RebuildChips()
					tp.UpdateDropLabel()
					tp.AppendLog("Imported shared file: " + path)
					if tabs != nil {
						tabs.SelectIndex(0)
					}
				}
			})
		}
	}()
	platsvc.PollShareIntent()
	go func() {
		ticker := time.NewTicker(750 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			platsvc.PollShareIntent()
		}
	}()
}

func buildTranscodeTabAndroid(tp *transcribe.Panel, settings *settingsPanel) fyne.CanvasObject {
	tp.TranscribeBtn.Importance = widget.HighImportance

	addBtn := newSecondaryButton("ADD FILES", tp.OnBrowse)
	cancelBtn := decor.NewDangerButton("CANCEL", tp.OnCancel)

	var recBtn *pointerButton
	recBtn = newPointerButtonWithIcon("RECORD", assets.MicIcon, func() { tp.OnToggleRecord(recBtn) })
	recBtn.Importance = widget.HighImportance

	settingsRow := container.NewGridWithColumns(3,
		newFormField("MODEL", settings.newModelSelectMirror()),
		newFormField("LANGUAGE", settings.newLangSelectMirror()),
		newFormField("SPEAKERS", settings.newSpeakersSelectMirror()),
	)

	actionRow := container.NewGridWithColumns(3,
		wrapAction(recBtn),
		wrapAction(addBtn),
		wrapAction(cancelBtn),
	)

	bottomBar := container.NewVBox(
		tp.Progress,
		container.NewBorder(nil, nil, tp.StatusText, nil),
		settingsRow,
		actionRow,
		vGap(spaceMD),
	)

	return container.NewBorder(
		nil, bottomBar, nil, nil,
		tp.Container,
	)
}

var permsSection *permissionsSection

func inlineField(label string, w fyne.CanvasObject) fyne.CanvasObject {
	lbl := canvas.NewText(label, decor.TextMuted)
	lbl.TextSize = textCaption
	lbl.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	left := container.New(&fixedWidthLayout{width: 90}, lbl)
	return container.NewBorder(nil, nil, left, nil, w)
}

func compactStatsLine(text string) fyne.CanvasObject {
	t := canvas.NewText(text, decor.TextSecondary)
	t.TextSize = textCaption
	t.TextStyle = fyne.TextStyle{Monospace: true}
	return t
}

func buildSettingsTab(sp *settingsPanel, deviceInfo, version string) fyne.CanvasObject {
	settingsGrid := container.NewVBox(
		inlineField("DEVICE", sp.deviceSelect),
		inlineField("THREADS", sp.threadsSelect),
		inlineField("CACHE", sp.expirySelect),
	)

	versionLabel := canvas.NewText(version, decor.TextMuted)
	versionLabel.TextSize = textCaption
	versionLabel.TextStyle = decor.MonoBoldStyle
	header := decor.NewPanelHeader(newCaptionText("SETTINGS"), versionLabel)

	_ = deviceInfo
	stats := deviceStats()
	statMap := map[string]string{}
	for _, st := range stats {
		statMap[st.Label] = st.Value
	}
	statsBlock := container.NewVBox(
		compactStatsLine(statMap["CPU"]+"  ·  "+statMap["RAM"]),
		compactStatsLine("GPU  "+statMap["GPU"]),
	)

	if permsSection == nil {
		permsSection = newPermissionsSection()
	}

	if sp.models == nil {
		sp.models = newModelsSection(sp.window)
	}

	toggleRow := container.NewGridWithColumns(2,
		wrapGhost(sp.noDiarizeBtn),
		wrapGhost(sp.debugBtn),
	)

	clearCacheBtn := newSecondaryButton("CLEAR CACHE", sp.onClearCache)
	clearTranscriptsBtn := newSecondaryButton("CLEAR TRANSCRIPTS", sp.onClearTranscripts)

	clearRow := container.NewGridWithColumns(2,
		wrapAction(clearCacheBtn),
		wrapAction(clearTranscriptsBtn),
	)
	saveRow := wrapAction(sp.saveBtn)

	bodySection := container.NewVBox(
		vGap(spaceSM),
		newSectionDivider(),
		vGap(spaceSM),
		statsBlock,
		vGap(spaceLG),
		settingsGrid,
		vGap(spaceLG),
		permsSection.container,
		vGap(spaceLG),
		newSectionDivider(),
		sp.models.container,
		vGap(spaceLG),
		newSectionDivider(),
		vGap(spaceMD),
		toggleRow,
		vGap(spaceMD),
		clearRow,
		vGap(spaceMD),
		saveRow,
		vGap(spaceLG),
	)

	pad := func(o fyne.CanvasObject) fyne.CanvasObject {
		return container.New(&insetLayout{padX: spaceXL, padY: 0}, o)
	}

	return container.NewBorder(
		header, nil, nil, nil,
		container.NewVScroll(pad(bodySection)),
	)
}
