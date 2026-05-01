package gui

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/asolopovas/wt/internal/gui/decor"
	"github.com/asolopovas/wt/internal/models"
)

type modelsSection struct {
	window    fyne.Window
	mgr       *models.Manager
	container *fyne.Container
	rows      *fyne.Container
	diskLabel *canvas.Text

	mu       sync.Mutex
	cancels  map[string]context.CancelFunc
	progress map[string]*widget.ProgressBar
}

func newModelsSection(win fyne.Window) *modelsSection {
	s := &modelsSection{
		window:   win,
		mgr:      models.NewManager(),
		cancels:  map[string]context.CancelFunc{},
		progress: map[string]*widget.ProgressBar{},
	}

	header := canvas.NewText("MODELS", decor.TextMuted)
	header.TextSize = textHeading
	header.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	s.diskLabel = canvas.NewText("", decor.TextMuted)
	s.diskLabel.TextSize = textCaption
	s.diskLabel.TextStyle = fyne.TextStyle{Monospace: true}

	head := container.NewBorder(nil, nil, header, s.diskLabel, nil)
	s.rows = container.NewVBox()
	s.container = container.NewVBox(head, vGap(spaceSM), s.rows)
	s.refresh()
	return s
}

func (s *modelsSection) refresh() {
	s.rows.Objects = nil
	s.diskLabel.Text = humanBytes(s.mgr.DiskUsage()) + " used"
	s.diskLabel.Refresh()

	families := []struct {
		f     models.Family
		title string
	}{
		{models.FamilyWhisper, "TRANSCRIPTION"},
		{models.FamilyDiarizer, "DIARIZATION"},
		{models.FamilyLLM, "LANGUAGE MODELS"},
	}

	first := true
	for _, fam := range families {
		entries := models.ByFamily(fam.f)
		if len(entries) == 0 {
			continue
		}
		if !first {
			s.rows.Add(vGap(spaceMD))
		}
		first = false

		sub := canvas.NewText(fam.title, decor.TextMuted)
		sub.TextSize = textCaption
		sub.TextStyle = monoBoldStyle
		s.rows.Add(sub)

		for _, e := range entries {
			s.rows.Add(s.buildRow(e))
		}
	}
	s.rows.Refresh()
	s.container.Refresh()
}

func (s *modelsSection) buildRow(e models.Entry) fyne.CanvasObject {
	status := s.mgr.Status(e.ID)
	isActive := s.mgr.Active(e.Family) == e.ID

	s.mu.Lock()
	_, downloading := s.cancels[e.ID]
	s.mu.Unlock()

	glyph, glyphCol := modelStatusGlyph(status, isActive, downloading)
	lead := canvas.NewText(glyph, glyphCol)
	lead.TextSize = textRow
	lead.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	leadBox := container.New(&fixedWidthLayoutModels{width: 16}, lead)

	nameCol := decor.TextPrimary
	if isActive {
		nameCol = decor.StatusActive
	}
	name := canvas.NewText(modelShortName(e.DisplayName), nameCol)
	name.TextSize = textBody
	name.TextStyle = fyne.TextStyle{Bold: true}

	size := canvas.NewText(humanBytes(e.SizeBytes), decor.TextMuted)
	size.TextSize = textCaption
	size.TextStyle = fyne.TextStyle{Monospace: true}

	info := container.NewBorder(nil, nil, leadBox, size, name)

	mkIconBtn := func(icon fyne.Resource, importance widget.Importance, onTap func()) fyne.CanvasObject {
		b := widget.NewButtonWithIcon("", icon, onTap)
		b.Importance = importance
		return container.New(&fixedWidthLayoutModels{width: iconBtnW}, b)
	}

	deleteBtn := func() fyne.CanvasObject {
		return mkIconBtn(theme.DeleteIcon(), widget.LowImportance, func() {
			showConfirm(s.window, "Delete model",
				fmt.Sprintf("Delete %s (%s)? You can re-download it from this panel later.", e.DisplayName, humanBytes(e.SizeBytes)),
				func() {
					if err := s.mgr.Delete(e.ID); err != nil {
						showError(s.window, err)
						return
					}
					fyne.Do(s.refresh)
				})
		})
	}

	var action fyne.CanvasObject
	switch {
	case downloading:
		bar := s.progress[e.ID]
		if bar == nil {
			bar = widget.NewProgressBar()
			s.progress[e.ID] = bar
		}
		action = mkIconBtn(theme.CancelIcon(), widget.MediumImportance, func() { s.cancel(e.ID) })
		row := container.NewBorder(nil, nil, nil, action, info)
		return container.NewVBox(row, bar)

	case status == models.StatusInstalled && !isActive:
		activate := mkIconBtn(theme.ConfirmIcon(), widget.HighImportance, func() {
			if err := s.mgr.SetActive(e.ID); err != nil {
				showError(s.window, err)
				return
			}
			fyne.Do(s.refresh)
		})
		action = container.NewHBox(activate, deleteBtn())

	case status == models.StatusInstalled && isActive:
		action = deleteBtn()

	default:
		action = mkIconBtn(downloadIcon, widget.HighImportance, func() { s.startDownload(e) })
	}

	return container.NewBorder(nil, nil, nil, action, info)
}

func modelStatusGlyph(st models.Status, active, downloading bool) (string, color.Color) {
	if downloading {
		return "↓", decor.ActionPrimary
	}
	switch {
	case active:
		return "●", decor.StatusActive
	case st == models.StatusInstalled:
		return "○", decor.TextMuted
	default:
		return "·", decor.TextMuted
	}
}

func modelShortName(s string) string {
	if i := strings.Index(s, " ("); i > 0 {
		return s[:i]
	}
	return s
}

const iconBtnW float32 = 32

func (s *modelsSection) startDownload(e models.Entry) {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	if _, exists := s.cancels[e.ID]; exists {
		s.mu.Unlock()
		cancel()
		return
	}
	s.cancels[e.ID] = cancel
	s.mu.Unlock()

	bar := widget.NewProgressBar()
	s.mu.Lock()
	s.progress[e.ID] = bar
	s.mu.Unlock()

	fyne.Do(s.refresh)

	go func() {
		err := s.mgr.Get(ctx, e.ID, func(p models.Progress) {
			if p.Total <= 0 {
				return
			}
			frac := float64(p.Downloaded) / float64(p.Total)
			if frac < 0 {
				frac = 0
			}
			if frac > 1 {
				frac = 1
			}
			fyne.Do(func() { bar.SetValue(frac) })
		})

		s.mu.Lock()
		delete(s.cancels, e.ID)
		delete(s.progress, e.ID)
		s.mu.Unlock()

		if err != nil && ctx.Err() == nil {
			fyne.Do(func() { showError(s.window, err) })
		}
		fyne.Do(s.refresh)
	}()
}

func (s *modelsSection) cancel(id string) {
	s.mu.Lock()
	cancel := s.cancels[id]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func humanBytes(n int64) string {
	const (
		kb int64 = 1 << 10
		mb       = 1 << 20
		gb       = 1 << 30
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(gb))
	case n >= mb:
		return fmt.Sprintf("%d MB", n/mb)
	case n >= kb:
		return fmt.Sprintf("%d KB", n/kb)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

type fixedWidthLayoutModels struct {
	width float32
}

func (l *fixedWidthLayoutModels) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		h := o.MinSize().Height
		if h > size.Height {
			h = size.Height
		}
		y := (size.Height - h) / 2
		o.Move(fyne.NewPos(0, y))
		o.Resize(fyne.NewSize(l.width, h))
	}
}

func (l *fixedWidthLayoutModels) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var h float32
	for _, o := range objs {
		m := o.MinSize()
		if m.Height > h {
			h = m.Height
		}
	}
	return fyne.NewSize(l.width, h)
}
