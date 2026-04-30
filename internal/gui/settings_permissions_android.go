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
	header.TextSize = 12
	header.TextStyle = fyne.TextStyle{Bold: true}
	header.Alignment = fyne.TextAlignCenter

	gap := canvas.NewRectangle(transparent)
	gap.SetMinSize(fyne.NewSize(0, 8))

	rows := container.NewVBox()
	root := container.NewVBox(header, gap, rows)
	s := &permissionsSection{container: root, rows: rows}
	s.refresh()
	return s
}

func (s *permissionsSection) refresh() {
	s.rows.Objects = nil

	for _, p := range collectPermissionInfos() {
		s.rows.Add(s.buildRow(p))
	}

	ignore := isIgnoringBatteryOptimizations()
	s.rows.Add(s.buildBatteryRow(ignore))

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

	label := canvas.NewText(p.label, colSecondary)
	label.TextSize = 12
	label.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	desc := widget.NewLabel(p.purpose)
	desc.TextStyle = fyne.TextStyle{Monospace: true}
	desc.Wrapping = fyne.TextWrapWord

	status := canvas.NewText(statusText, statusColor)
	status.TextSize = 11
	status.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	status.Alignment = fyne.TextAlignTrailing

	id := p.id
	granted := p.granted
	btnText := "ENABLE"
	if granted {
		btnText = "MANAGE"
	}
	btn := newPointerButton(btnText, func() {
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

	header := container.NewBorder(nil, nil, label, status)
	row := container.NewVBox(header, desc, borderedBtn(btn, colOutline))
	return container.NewPadded(row)
}

func (s *permissionsSection) buildBatteryRow(ignoring bool) fyne.CanvasObject {
	statusColor := colError
	statusText := "RESTRICTED"
	if ignoring {
		statusColor = colSuccess
		statusText = "UNRESTRICTED"
	}

	label := canvas.NewText("BATTERY", colSecondary)
	label.TextSize = 12
	label.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	desc := widget.NewLabel("Skip Doze battery limit.")
	desc.TextStyle = fyne.TextStyle{Monospace: true}
	desc.Wrapping = fyne.TextWrapWord

	status := canvas.NewText(statusText, statusColor)
	status.TextSize = 11
	status.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	status.Alignment = fyne.TextAlignTrailing

	btn := newPointerButton("OPEN SETTINGS", func() {
		openBatteryOptimizationSettings()
	})
	if ignoring {
		btn.Importance = widget.LowImportance
	} else {
		btn.Importance = widget.HighImportance
	}

	header := container.NewBorder(nil, nil, label, status)
	row := container.NewVBox(header, desc, borderedBtn(btn, colOutline))
	return container.NewPadded(row)
}
