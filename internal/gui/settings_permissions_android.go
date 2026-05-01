//go:build android

package gui

import (
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/asolopovas/wt/internal/gui/decor"
	"github.com/asolopovas/wt/internal/gui/platsvc"
)

var (
	permRequestedMu sync.Mutex
	permRequested   = map[string]bool{}
)

func markPermRequested(id string) {
	permRequestedMu.Lock()
	permRequested[id] = true
	permRequestedMu.Unlock()
}

func wasPermRequested(id string) bool {
	permRequestedMu.Lock()
	defer permRequestedMu.Unlock()
	return permRequested[id]
}

type permissionsSection struct {
	container *fyne.Container
	rows      *fyne.Container
}

func newPermissionsSection() *permissionsSection {
	header := canvas.NewText("PERMISSIONS", decor.TextMuted)
	header.TextSize = textHeading
	header.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	rows := container.NewVBox()
	root := container.NewVBox(header, vGap(spaceXS), rows)
	s := &permissionsSection{container: root, rows: rows}
	s.refresh()
	return s
}

func (s *permissionsSection) refresh() {
	s.rows.Objects = nil

	cells := []fyne.CanvasObject{}
	for _, p := range platsvc.CollectPermissionInfos() {
		cells = append(cells, s.buildCell(p))
	}
	cells = append(cells, s.buildBatteryCell(platsvc.IsIgnoringBatteryOptimizations()))

	grid := container.NewGridWithColumns(2, cells...)
	s.rows.Add(grid)

	s.rows.Refresh()
	s.container.Refresh()
}

func (s *permissionsSection) buildCell(p platsvc.PermissionInfo) fyne.CanvasObject {
	id := p.ID
	granted := p.Granted
	label := p.Label
	return s.assembleCell(label, granted, func(want bool) {
		if !want {
			platsvc.OpenAppSettings()
			return
		}
		if wasPermRequested(id) && !platsvc.ShouldShowPermissionRationale(id) {
			platsvc.OpenAppSettings()
			return
		}
		markPermRequested(id)
		platsvc.RequestPermissions([]string{id})
		go func() {
			platsvc.PollPermission(id, func() { fyne.Do(s.refresh) })
		}()
	})
}

func (s *permissionsSection) buildBatteryCell(ignoring bool) fyne.CanvasObject {
	return s.assembleCell("BATTERY", ignoring, func(bool) {
		platsvc.OpenBatteryOptimizationSettings()
	})
}

func (s *permissionsSection) assembleCell(label string, ok bool, action func(want bool)) fyne.CanvasObject {
	mark := "✓ "
	if !ok {
		mark = "× "
	}
	btn := newPointerButton(mark+label, func() {
		action(!ok)
		fyne.Do(s.refresh)
	})
	if ok {
		btn.Importance = widget.LowImportance
	} else {
		btn.Importance = widget.DangerImportance
	}
	return wrapGhost(btn)
}

type fixedWidthLayout struct {
	width float32
}

func (l *fixedWidthLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(fyne.NewSize(l.width, size.Height))
	}
}

func (l *fixedWidthLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var h float32
	for _, o := range objs {
		m := o.MinSize()
		if m.Height > h {
			h = m.Height
		}
	}
	return fyne.NewSize(l.width, h)
}

type insetLayout struct {
	padX, padY float32
}

func (l *insetLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(l.padX, l.padY))
		o.Resize(fyne.NewSize(size.Width-2*l.padX, size.Height-2*l.padY))
	}
}

func (l *insetLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objs {
		m := o.MinSize()
		if m.Width > w {
			w = m.Width
		}
		if m.Height > h {
			h = m.Height
		}
	}
	return fyne.NewSize(w+2*l.padX, h+2*l.padY)
}
