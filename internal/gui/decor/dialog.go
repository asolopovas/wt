package decor

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type ButtonKind int

const (
	KindSecondary ButtonKind = iota
	KindPrimary
	KindDanger
)

type DialogAction struct {
	Label string
	OnTap func()
	Kind  ButtonKind
}

type DialogConfig struct {
	Parent      fyne.Window
	Title       string
	Body        fyne.CanvasObject
	Actions     []DialogAction
	WidthFrac   float32
	Size        *fyne.Size
	TopInset    float32
	BottomInset float32
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
			if hide != nil {
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
		top = container.NewVBox(topGap, container.NewHBox(NewSectionHeader(cfg.Title)))
	}

	bodyContainer := container.NewBorder(top, bottom, nil, nil, cfg.Body)
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
