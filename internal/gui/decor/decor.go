package decor

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

func VGap(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(Transparent)
	r.SetMinSize(fyne.NewSize(0, h))
	return r
}

func HGap(w float32) fyne.CanvasObject {
	r := canvas.NewRectangle(Transparent)
	r.SetMinSize(fyne.NewSize(w, 0))
	return r
}

func DialogBordered(content fyne.CanvasObject) fyne.CanvasObject {
	frame := canvas.NewRectangle(Transparent)
	frame.StrokeColor = BorderStrong
	frame.StrokeWidth = 1

	inner := container.NewBorder(
		VGap(SpaceXXL), VGap(SpaceXXL),
		HGap(SpaceXXL), HGap(SpaceXXL),
		content,
	)
	return container.NewStack(frame, inner)
}

func borderedBtn(btn fyne.CanvasObject, borderCol color.Color) fyne.CanvasObject {
	frame := canvas.NewRectangle(Transparent)
	frame.StrokeColor = borderCol
	frame.StrokeWidth = 1
	return container.NewStack(frame, btn)
}
