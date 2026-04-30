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
	"github.com/asolopovas/wt/internal/gui/assets"
	"github.com/asolopovas/wt/internal/gui/cache"
	"github.com/asolopovas/wt/internal/gui/decor"
	"github.com/asolopovas/wt/internal/gui/platsvc"
	"github.com/asolopovas/wt/internal/gui/transcribe"
)

func Run(version string) error {
	cfg, _ := shared.Load()

	a := app.New()
	a.SetIcon(appIcon)
	a.Settings().SetTheme(&whisperTheme{})

	w := a.NewWindow("wt " + version)
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
	settingsTab := buildSettingsTab(settings, deviceInfo)

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
		newFormField("MODEL", settings.modelSelect),
		newFormField("LANGUAGE", settings.langSelect),
		newFormField("SPEAKERS", settings.speakersSelect),
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

func buildSettingsTab(sp *settingsPanel, deviceInfo string) fyne.CanvasObject {
	settingsGrid := container.NewGridWithColumns(2,
		newFormField("DEVICE", sp.deviceSelect),
		newFormField("THREADS", sp.threadsSelect),
		newFormField("EXPIRY (DAYS)", sp.expirySelect),
	)

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
		vGap(spaceXL),
		header,
		vGap(spaceXXL),
		settingsGrid,
		vGap(spaceXL),
		deviceHeader,
		deviceLabel,
		vGap(spaceXXL),
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

	bottomSection := container.NewVBox(
		toggleRow,
		cacheRow,
		actionRow,
		vGap(spaceMD),
	)

	return container.NewBorder(
		nil, bottomSection, nil, nil,
		container.NewVScroll(topSection),
	)
}
