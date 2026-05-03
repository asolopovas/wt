//go:build android

package gui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/fsnotify/fsnotify"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/appinfo"
	"github.com/asolopovas/wt/internal/gui/assets"
	"github.com/asolopovas/wt/internal/gui/decor"
	"github.com/asolopovas/wt/internal/gui/platsvc"
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

	settings := newSettingsPanel(cfg, w)
	tp := transcribe.New(w, settings)
	dock := newPlayerDock(w, tp)
	history := newHistoryPanel(w, tp, dock)
	tp.History = history
	attachLibrary(tp, history)
	if history.headerRight != nil {
		sep := canvas.NewText(" | ", colMuted)
		sep.TextSize = textBody
		sep.TextStyle = fyne.TextStyle{Monospace: true}
		sepWrap := container.NewCenter(sep)
		sepWrap.Hide()
		tp.TimerSep = sepWrap
		history.headerRight.Objects = []fyne.CanvasObject{tp.StatsLine, sepWrap, tp.TimerText}
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
	go watchImportsDir(history.Refresh)

	deviceInfo := detectDevice()

	transcodeTab := buildTranscodeTabAndroid(tp, settings)
	settingsTab := buildSettingsTab(settings, deviceInfo, versionLabel(version, buildDate))

	if missing := platsvc.MissingPermissions(); len(missing) > 0 {
		go func(p []string) {
			time.Sleep(600 * time.Millisecond)
			fyne.Do(func() { platsvc.RequestPermissions(p) })
		}(missing)
	}
	if !platsvc.IsExternalStorageManager() {
		go func() {
			time.Sleep(1500 * time.Millisecond)
			fyne.Do(func() {
				decor.ShowConfirm(w, "All-Files Access",
					"To detect audio files placed in Documents/WTranscribe by other apps (file manager, adb, etc.), grant 'All files access'. Open settings now?",
					func() { platsvc.OpenAllFilesAccessSettings() })
			})
		}()
	}

	settingsTabItem := container.NewTabItem("SETTINGS", settingsTab)
	// LOG tab was removed — the persistent run log lives at
	// <MediaDir>/wt.log and is reachable via Settings → VIEW LOG. The
	// in-app live log was duplicating that surface.
	tabs := container.NewAppTabs(
		container.NewTabItem("TRANSCODE", transcodeTab),
		settingsTabItem,
	)
	tabs.SetTabLocation(container.TabLocationBottom)
	tabs.OnSelected = func(t *container.TabItem) {
		if t == settingsTabItem && permsSection != nil {
			permsSection.refresh()
		}
	}

	rebuildContent := func() {
		w.SetContent(container.NewBorder(nil, dock.Container(), nil, nil, tabs))
	}
	dock.onVisibilityChange = rebuildContent
	rebuildContent()
	wireShareIntake(tp, tabs)
	w.ShowAndRun()
	return nil
}

var audioExts = map[string]bool{
	".m4a": true, ".mp3": true, ".wav": true, ".flac": true,
	".ogg": true, ".opus": true, ".aac": true, ".webm": true, ".mp4": true,
}

func reconcileImports(dir string) {
	seen := map[string]struct{}{}
	add := func(path string) {
		if path == "" {
			return
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		if _, dup := seen[abs]; dup {
			return
		}
		ext := strings.ToLower(filepath.Ext(abs))
		if !audioExts[ext] {
			return
		}
		if strings.HasPrefix(filepath.Base(abs), ".") {
			return
		}
		seen[abs] = struct{}{}
		_, _ = cache.StorePending(abs)
	}

	if ents, err := os.ReadDir(dir); err == nil {
		for _, ent := range ents {
			if ent.IsDir() {
				continue
			}
			add(filepath.Join(dir, ent.Name()))
		}
	}

	for _, p := range platsvc.QueryAudioFilesIn(dir) {
		add(p)
	}
}

func watchImportsDir(refresh func()) {
	imports := shared.MediaDir()
	if err := os.MkdirAll(imports, 0o755); err != nil {
		return
	}
	platsvc.RescanMediaDir(imports)
	reconcileImports(imports)
	fyne.Do(refresh)
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for range t.C {
			platsvc.RescanMediaDir(imports)
			reconcileImports(imports)
			fyne.Do(refresh)
		}
	}()
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	if err := w.Add(imports); err != nil {
		_ = w.Close()
		return
	}
	var pending bool
	var timer *time.Timer
	fire := func() {
		pending = false
		reconcileImports(imports)
		fyne.Do(refresh)
	}
	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if ev.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename|fsnotify.Write) == 0 {
				continue
			}
			if !pending {
				pending = true
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(300*time.Millisecond, fire)
			}
		case _, ok := <-w.Errors:
			if !ok {
				return
			}
		}
	}
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
	cancelBtn.Disable()
	tp.CancelBtn = cancelBtn

	var recBtn *pointerButton
	recBtn = newPointerButtonWithIcon("RECORD", assets.MicIcon, func() { tp.OnToggleRecord(recBtn) })
	recBtn.Importance = widget.DangerImportance

	settingsRow := container.NewGridWithColumns(2,
		newFormField("MODEL", settings.newModelSelectMirror()),
		newFormField("DIARIZER", settings.newDiarizerSelectMirror()),
		newFormField("LANGUAGE", settings.newLangSelectMirror()),
		newFormField("SPEAKERS", settings.newSpeakersSelectMirror()),
	)

	actionRow := container.NewGridWithColumns(3,
		wrapAction(cancelBtn),
		wrapAction(addBtn),
		wrapAction(recBtn),
	)

	bottomBar := container.NewVBox(
		tp.Progress,
		container.NewBorder(nil, nil, tp.StatusText, nil),
		settingsRow,
		vGap(spaceXXL),
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
		inlineField("CACHE EXPIRY", sp.expirySelect),
		inlineField("LOG RETENTION", sp.logRetainSelect),
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
	viewLogBtn := newSecondaryButton("VIEW LOG", sp.onViewLog)
	clearLogBtn := newSecondaryButton("CLEAR LOG", sp.onClearLog)

	clearRow := container.NewGridWithColumns(2,
		wrapAction(clearCacheBtn),
		wrapAction(clearTranscriptsBtn),
	)
	logRow := container.NewGridWithColumns(2,
		wrapAction(viewLogBtn),
		wrapAction(clearLogBtn),
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
		logRow,
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
