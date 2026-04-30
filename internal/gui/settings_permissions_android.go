//go:build android

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type permissionsSection struct {
	container *fyne.Container
	rows      *fyne.Container
}

func newPermissionsSection() *permissionsSection {
	header := canvas.NewText("PERMISSIONS", colMuted)
	header.TextSize = textLabel
	header.TextStyle = fyne.TextStyle{Bold: true}
	header.Alignment = fyne.TextAlignCenter

	rows := container.NewVBox()
	root := container.NewVBox(header, vGap(spaceLG), rows)
	s := &permissionsSection{container: root, rows: rows}
	s.refresh()
	return s
}

func (s *permissionsSection) refresh() {
	s.rows.Objects = nil

	cells := []fyne.CanvasObject{}
	for _, p := range collectPermissionInfos() {
		cells = append(cells, s.buildRow(p))
	}
	cells = append(cells, s.buildBatteryRow(isIgnoringBatteryOptimizations()))

	for i := 0; i < len(cells); i += 2 {
		left := cells[i]
		var right fyne.CanvasObject = canvas.NewRectangle(transparent)
		if i+1 < len(cells) {
			right = cells[i+1]
		}
		s.rows.Add(container.NewGridWithColumns(2, left, right))
	}

	s.rows.Refresh()
	s.container.Refresh()
}

func (s *permissionsSection) buildRow(p permissionInfo) fyne.CanvasObject {
	statusColor := colError
	statusText := "DISABLED"
	if p.granted {
		statusColor = colSuccess
		statusText = "ENABLED"
	}

	desc := widget.NewLabel(p.purpose)
	desc.TextStyle = fyne.TextStyle{Monospace: true}
	desc.Wrapping = fyne.TextWrapWord

	status := canvas.NewText(statusText, statusColor)
	status.TextSize = textBody
	status.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	status.Alignment = fyne.TextAlignCenter

	id := p.id
	granted := p.granted
	btn := newPointerButton(p.label, func() {
		if granted {
			openAppSettings()
			return
		}
		requestPermissions([]string{id})

		go func() {
			pollPermission(id, func() { fyne.Do(s.refresh) })
		}()
	})
	if granted {
		btn.Importance = widget.LowImportance
	} else {
		btn.Importance = widget.HighImportance
	}

	row := container.NewVBox(status, desc, wrapAction(btn))
	return container.NewPadded(row)
}

func (s *permissionsSection) buildBatteryRow(ignoring bool) fyne.CanvasObject {
	statusColor := colError
	statusText := "RESTRICTED"
	if ignoring {
		statusColor = colSuccess
		statusText = "UNRESTRICTED"
	}

	desc := widget.NewLabel("Skip Doze battery limit.")
	desc.TextStyle = fyne.TextStyle{Monospace: true}
	desc.Wrapping = fyne.TextWrapWord

	status := canvas.NewText(statusText, statusColor)
	status.TextSize = textBody
	status.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	status.Alignment = fyne.TextAlignCenter

	btn := newPointerButton("BATTERY", func() {
		openBatteryOptimizationSettings()
	})
	if ignoring {
		btn.Importance = widget.LowImportance
	} else {
		btn.Importance = widget.HighImportance
	}

	row := container.NewVBox(status, desc, wrapAction(btn))
	return container.NewPadded(row)
}
