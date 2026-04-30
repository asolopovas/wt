//go:build android

package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/asolopovas/wt/internal/gui/decor"
	"github.com/asolopovas/wt/internal/gui/platsvc"
)

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

	first := true
	addRow := func(row fyne.CanvasObject) {
		if !first {
			s.rows.Add(vGap(spaceXS))
		}
		s.rows.Add(row)
		first = false
	}

	for _, p := range platsvc.CollectPermissionInfos() {
		addRow(s.buildRow(p))
	}
	addRow(s.buildBatteryRow(platsvc.IsIgnoringBatteryOptimizations()))

	s.rows.Refresh()
	s.container.Refresh()
}

func (s *permissionsSection) buildRow(p platsvc.PermissionInfo) fyne.CanvasObject {
	id := p.ID
	granted := p.Granted
	return s.assembleRow(p.Label, granted, func(want bool) {
		if want {
			platsvc.RequestPermissions([]string{id})
			go func() {
				platsvc.PollPermission(id, func() { fyne.Do(s.refresh) })
			}()
		} else {
			platsvc.OpenAppSettings()
		}
	})
}

func (s *permissionsSection) buildBatteryRow(ignoring bool) fyne.CanvasObject {
	return s.assembleRow("BATTERY", ignoring, func(bool) {
		platsvc.OpenBatteryOptimizationSettings()
	})
}

func (s *permissionsSection) assembleRow(label string, ok bool, action func(want bool)) fyne.CanvasObject {
	title := canvas.NewText(label, color.White)
	title.TextSize = textCaption
	title.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	btnLabel := "OFF"
	if ok {
		btnLabel = "ON"
	}
	btn := newPointerButton(btnLabel, func() {
		action(!ok)
		fyne.Do(s.refresh)
	})
	if ok {
		btn.Importance = widget.LowImportance
	} else {
		btn.Importance = widget.HighImportance
	}
	action_ := container.New(&fixedWidthLayout{width: 70}, wrapAction(btn))

	return container.New(layout.NewHBoxLayout(), title, layout.NewSpacer(), action_)
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
