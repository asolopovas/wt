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

	mu            sync.Mutex
	cancels       map[string]context.CancelFunc
	progress      map[string]*widget.ProgressBar
	dialogRefresh func()
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

	addBtn := widget.NewButtonWithIcon("ADD", theme.ContentAddIcon(), s.openDownloadDialog)
	addBtn.Importance = widget.LowImportance

	right := container.NewHBox(s.diskLabel, addBtn)
	head := container.NewBorder(nil, nil, header, right, nil)
	s.rows = container.New(&tightVBox{gap: 0})
	s.container = container.NewVBox(head, vGap(spaceSM), s.rows)
	s.refresh()
	return s
}

type iconTap struct {
	widget.BaseWidget
	icon  fyne.Resource
	size  float32
	onTap func()
}

func newIconTap(icon fyne.Resource, size float32, onTap func()) *iconTap {
	t := &iconTap{icon: icon, size: size, onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}

func (t *iconTap) CreateRenderer() fyne.WidgetRenderer {
	img := canvas.NewImageFromResource(t.icon)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(t.size, t.size))
	return widget.NewSimpleRenderer(container.New(&fixedSquareLayout{size: t.size + spaceMD*2, inner: t.size}, img))
}

func (t *iconTap) MinSize() fyne.Size {
	w := t.size + spaceMD*2
	return fyne.NewSize(w, w)
}

func (t *iconTap) Tapped(_ *fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

type fixedSquareLayout struct {
	size  float32
	inner float32
}

func (l *fixedSquareLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos((size.Width-l.inner)/2, (size.Height-l.inner)/2))
		o.Resize(fyne.NewSize(l.inner, l.inner))
	}
}

func (l *fixedSquareLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(l.size, l.size)
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
		var visible []models.Entry
		for _, e := range models.ByFamily(fam.f) {
			st := s.mgr.Status(e.ID)
			s.mu.Lock()
			_, dl := s.cancels[e.ID]
			s.mu.Unlock()
			if st == models.StatusInstalled || dl {
				visible = append(visible, e)
			}
		}
		if len(visible) == 0 {
			continue
		}
		if !first {
			s.rows.Add(vGap(20))
		}
		first = false

		sub := canvas.NewText(fam.title, decor.TextMuted)
		sub.TextSize = textCaption
		sub.TextStyle = monoBoldStyle
		s.rows.Add(sub)
		s.rows.Add(vGap(20))

		for _, e := range visible {
			s.rows.Add(s.buildRow(e))
		}
	}
	if len(s.rows.Objects) == 0 {
		empty := canvas.NewText("No models installed — tap ADD to download.", decor.TextMuted)
		empty.TextSize = textCaption
		empty.TextStyle = fyne.TextStyle{Monospace: true}
		s.rows.Add(empty)
	}
	s.rows.Refresh()
	s.container.Refresh()
	if s.dialogRefresh != nil {
		s.dialogRefresh()
	}
}

func (s *modelsSection) buildRow(e models.Entry) fyne.CanvasObject {
	status := s.mgr.Status(e.ID)
	isActive := s.mgr.Active(e.Family) == e.ID

	s.mu.Lock()
	_, downloading := s.cancels[e.ID]
	s.mu.Unlock()

	info := s.modelInfoBlock(e, status, isActive, downloading)

	if downloading {
		bar := s.progress[e.ID]
		if bar == nil {
			bar = widget.NewProgressBar()
			s.progress[e.ID] = bar
		}
		cancel := newIconTap(theme.CancelIcon(), 18, func() { s.cancel(e.ID) })
		row := container.NewBorder(nil, nil, nil, cancel, info)
		return container.NewVBox(row, progressBarFixed(bar))
	}

	del := newIconTap(theme.DeleteIcon(), 18, func() {
		showConfirm(s.window, "Delete model",
			fmt.Sprintf("Delete %s (%s)?", e.DisplayName, humanBytes(e.SizeBytes)),
			func() {
				if err := s.mgr.Delete(e.ID); err != nil {
					showError(s.window, err)
					return
				}
				fyne.Do(s.refresh)
			})
	})

	row := container.NewBorder(nil, nil, nil, del, info)
	if isActive {
		return row
	}
	return newTappableRow(row, func() {
		if err := s.mgr.SetActive(e.ID); err != nil {
			showError(s.window, err)
			return
		}
		fyne.Do(s.refresh)
	})
}

func (s *modelsSection) modelInfoBlock(e models.Entry, status models.Status, isActive, downloading bool) fyne.CanvasObject {
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

	return container.NewBorder(nil, nil, leadBox, size, name)
}

type tappableRow struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onTap   func()
}

func newTappableRow(content fyne.CanvasObject, onTap func()) *tappableRow {
	r := &tappableRow{content: content, onTap: onTap}
	r.ExtendBaseWidget(r)
	return r
}

func (r *tappableRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.content)
}

func (r *tappableRow) Tapped(_ *fyne.PointEvent) {
	if r.onTap != nil {
		r.onTap()
	}
}

func (s *modelsSection) openDownloadDialog() {
	body := container.NewVBox()

	rebuild := func() {
		body.Objects = nil
		families := []struct {
			f     models.Family
			title string
		}{
			{models.FamilyWhisper, "TRANSCRIPTION"},
			{models.FamilyDiarizer, "DIARIZATION"},
			{models.FamilyLLM, "LANGUAGE MODELS"},
		}
		any := false
		first := true
		for _, fam := range families {
			var available []models.Entry
			for _, e := range models.ByFamily(fam.f) {
				if s.mgr.Status(e.ID) != models.StatusInstalled {
					available = append(available, e)
				}
			}
			if len(available) == 0 {
				continue
			}
			any = true
			if !first {
				body.Add(vGap(spaceMD))
			}
			first = false
			h := canvas.NewText(fam.title, decor.TextMuted)
			h.TextSize = textCaption
			h.TextStyle = monoBoldStyle
			body.Add(h)
			for _, e := range available {
				body.Add(s.buildDownloadRow(e))
			}
		}
		if !any {
			t := canvas.NewText("All available models are installed.", decor.TextMuted)
			t.TextSize = textCaption
			t.TextStyle = fyne.TextStyle{Monospace: true}
			body.Add(t)
		}
		body.Refresh()
	}
	rebuild()

	scroll := container.NewVScroll(body)
	scroll.SetMinSize(fyne.NewSize(280, 400))

	s.dialogRefresh = func() { fyne.Do(rebuild) }

	hide := showDialog(dialogConfig{
		Parent: s.window,
		Title:  "DOWNLOAD MODEL",
		Body:   scroll,
		Actions: []dialogAction{
			{Label: "DONE", Kind: kindPrimary},
		},
	})
	_ = hide
}

func (s *modelsSection) buildDownloadRow(e models.Entry) fyne.CanvasObject {
	status := s.mgr.Status(e.ID)
	s.mu.Lock()
	_, downloading := s.cancels[e.ID]
	s.mu.Unlock()

	info := s.modelInfoBlock(e, status, false, downloading)

	if downloading {
		bar := s.progress[e.ID]
		if bar == nil {
			bar = widget.NewProgressBar()
			s.progress[e.ID] = bar
		}
		cancel := newIconTap(theme.CancelIcon(), 18, func() { s.cancel(e.ID) })
		row := container.NewBorder(nil, nil, nil, cancel, info)
		return container.NewVBox(row, progressBarFixed(bar))
	}

	dl := newIconTap(downloadIcon, 18, func() { s.startDownload(e) })
	return container.NewBorder(nil, nil, nil, dl, info)
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

type fixedHeightLayout struct {
	height float32
}

func (l *fixedHeightLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(fyne.NewSize(size.Width, l.height))
	}
}

func (l *fixedHeightLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w float32
	for _, o := range objs {
		if m := o.MinSize().Width; m > w {
			w = m
		}
	}
	return fyne.NewSize(w, l.height)
}

func progressBarFixed(bar *widget.ProgressBar) fyne.CanvasObject {
	return container.New(&fixedHeightLayout{height: 32}, bar)
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
