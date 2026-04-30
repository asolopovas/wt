package gui

import (
	"context"
	"fmt"
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
	s.diskLabel.Text = fmt.Sprintf("STORAGE: %s", humanBytes(s.mgr.DiskUsage()))
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
		if !first {
			s.rows.Add(vGap(spaceMD))
		}
		first = false

		sub := canvas.NewText(fam.title, decor.TextSecondary)
		sub.TextSize = textCaption
		sub.TextStyle = monoBoldStyle
		s.rows.Add(sub)
		s.rows.Add(vGap(spaceXS))

		for _, e := range models.ByFamily(fam.f) {
			s.rows.Add(s.buildRow(e))
			s.rows.Add(vGap(spaceXS))
		}
	}
	s.rows.Refresh()
	s.container.Refresh()
}

func (s *modelsSection) buildRow(e models.Entry) fyne.CanvasObject {
	status := s.mgr.Status(e.ID)
	isActive := s.mgr.Active(e.Family) == e.ID

	name := widget.NewLabel(e.DisplayName)
	name.TextStyle = fyne.TextStyle{Bold: true}
	name.Truncation = fyne.TextTruncateEllipsis

	sub := widget.NewLabel(fmt.Sprintf("%s · %s", humanBytes(e.SizeBytes), statusText(status, isActive)))
	sub.TextStyle = fyne.TextStyle{Monospace: true}
	sub.Truncation = fyne.TextTruncateEllipsis

	info := container.NewVBox(name, sub)

	s.mu.Lock()
	_, downloading := s.cancels[e.ID]
	s.mu.Unlock()

	mkIconBtn := func(icon fyne.Resource, importance widget.Importance, onTap func()) fyne.CanvasObject {
		b := widget.NewButtonWithIcon("", icon, onTap)
		b.Importance = importance
		return container.New(&fixedWidthLayoutModels{width: iconBtnW}, b)
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

	case status == models.StatusInstalled:
		var btns []fyne.CanvasObject
		if !isActive {
			btns = append(btns, mkIconBtn(theme.ConfirmIcon(), widget.HighImportance, func() {
				if err := s.mgr.SetActive(e.ID); err != nil {
					showError(s.window, err)
					return
				}
				fyne.Do(s.refresh)
			}))
		}
		btns = append(btns, mkIconBtn(theme.CancelIcon(), widget.DangerImportance, func() {
			showConfirm(s.window, "Delete model",
				fmt.Sprintf("Delete %s (%s)? You can re-download it from this panel later.", e.DisplayName, humanBytes(e.SizeBytes)),
				func() {
					if err := s.mgr.Delete(e.ID); err != nil {
						showError(s.window, err)
						return
					}
					fyne.Do(s.refresh)
				})
		}))
		action = container.NewHBox(btns...)

	default:
		action = mkIconBtn(downloadIcon, widget.HighImportance, func() { s.startDownload(e) })
	}

	return container.NewBorder(nil, nil, nil, action, info)
}

const iconBtnW float32 = 44

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

func statusText(st models.Status, active bool) string {
	switch st {
	case models.StatusDownloading:
		return "DOWNLOADING"
	case models.StatusInstalled:
		if active {
			return "ACTIVE"
		}
		return "INSTALLED"
	default:
		return "NOT INSTALLED"
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
