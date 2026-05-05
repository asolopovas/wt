package decor

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type ButtonKind int

const (
	KindSecondary ButtonKind = iota
	KindPrimary
	KindDanger
)

type DialogAction struct {
	Label    string
	OnTap    func()
	Kind     ButtonKind
	KeepOpen bool
}

type DialogConfig struct {
	Parent      fyne.Window
	Title       string
	TitleRight  fyne.CanvasObject
	Body        fyne.CanvasObject
	Actions     []DialogAction
	WidthFrac   float32
	Size        *fyne.Size
	TopInset    float32
	BottomInset float32

	AnchorTop bool
}

func ShowDialog(cfg DialogConfig) func() {
	if cfg.Parent == nil {
		return func() {}
	}

	var hide func()

	actionObjs := make([]fyne.CanvasObject, 0, len(cfg.Actions))
	for _, a := range cfg.Actions {
		action := a
		var btn *PointerButton
		switch action.Kind {
		case KindPrimary:
			btn = NewPrimaryButton(action.Label, nil)
		case KindDanger:
			btn = NewDangerButton(action.Label, nil)
		default:
			btn = NewSecondaryButton(action.Label, nil)
		}
		btn.OnTapped = func() {
			if !action.KeepOpen && hide != nil {
				hide()
			}
			if action.OnTap != nil {
				action.OnTap()
			}
		}
		actionObjs = append(actionObjs, WrapAction(btn))
	}

	bottomGap := VGap(cfg.BottomInset)
	bottom := bottomGap
	if len(actionObjs) > 0 {
		row := container.NewGridWithColumns(len(actionObjs), actionObjs...)
		bottom = container.NewVBox(row, bottomGap)
	}

	topGap := VGap(cfg.TopInset)
	top := topGap
	if cfg.Title != "" {
		header := NewSectionHeader(cfg.Title)
		var row fyne.CanvasObject
		if cfg.TitleRight != nil {
			row = container.New(layout.NewHBoxLayout(), header, layout.NewSpacer(), cfg.TitleRight)
		} else {
			row = container.NewHBox(header)
		}
		top = container.NewVBox(topGap, row)
	}

	bodyContainer := container.NewBorder(top, bottom, nil, nil, cfg.Body)

	if cfg.AnchorTop {
		hide = showAnchoredDialog(cfg, bodyContainer)
		return hide
	}

	pop := widget.NewModalPopUp(DialogBordered(bodyContainer), cfg.Parent.Canvas())

	if cfg.Size != nil {
		pop.Resize(*cfg.Size)
	} else if cfg.WidthFrac > 0 {
		winSize := cfg.Parent.Canvas().Size()
		pop.Resize(fyne.NewSize(winSize.Width*cfg.WidthFrac, pop.MinSize().Height))
	}

	hide = pop.Hide
	pop.Show()
	return hide
}

func showAnchoredDialog(cfg DialogConfig, bodyContainer fyne.CanvasObject) func() {
	c := cfg.Parent.Canvas()

	th := fyne.CurrentApp().Settings().Theme()
	var shadow color.Color = color.NRGBA{A: 0xa8}
	if th != nil {
		v := fyne.CurrentApp().Settings().ThemeVariant()
		shadow = th.Color(theme.ColorNameShadow, v)
	}
	underlay := canvas.NewRectangle(shadow)
	underlay.Resize(c.Size())
	underlay.Move(fyne.NewPos(0, 0))

	pop := widget.NewPopUp(DialogBordered(bodyContainer), c)

	canvasSize := c.Size()
	areaPos, areaSize := c.InteractiveArea()

	var w, h float32
	if cfg.Size != nil {
		w = cfg.Size.Width
		h = cfg.Size.Height
	} else {
		widthFrac := cfg.WidthFrac
		if widthFrac <= 0 {
			widthFrac = 0.92
		}
		w = canvasSize.Width * widthFrac
		h = pop.MinSize().Height
	}
	if w > areaSize.Width {
		w = areaSize.Width
	}
	if h > areaSize.Height {
		h = areaSize.Height
	}
	pop.Resize(fyne.NewSize(w, h))

	actualW := pop.MinSize().Width
	if actualW < w {
		actualW = w
	}
	if actualW > areaSize.Width {
		actualW = areaSize.Width
		pop.Resize(fyne.NewSize(actualW, h))
	}

	topMargin := float32(SpaceXL)
	x := areaPos.X + (areaSize.Width-actualW)/2
	if x < 0 {
		x = 0
	}
	y := areaPos.Y + topMargin
	pop.Move(fyne.NewPos(x, y))

	overlays := c.Overlays()
	overlays.Add(underlay)
	pop.Show()

	hidden := false
	hide := func() {
		if hidden {
			return
		}
		hidden = true
		pop.Hide()
		overlays.Remove(underlay)
	}
	return hide
}
