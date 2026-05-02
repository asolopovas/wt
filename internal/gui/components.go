package gui

import (
	"github.com/asolopovas/wt/internal/gui/decor"
	"github.com/asolopovas/wt/internal/gui/preview"
)

var (
	newPrimaryButton   = decor.NewPrimaryButton
	newSecondaryButton = decor.NewSecondaryButton
	wrapAction         = decor.WrapAction
	wrapGhost          = decor.WrapGhost
	newSectionDivider  = decor.NewSectionDivider
	newFormField       = decor.NewFormField
	newCaptionText     = decor.NewCaptionText
	newPanelBackground = decor.NewPanelBackground
	vGap               = decor.VGap
)

type (
	dialogAction = decor.DialogAction
	dialogConfig = decor.DialogConfig
)

const (
	kindSecondary = decor.KindSecondary
	kindPrimary   = decor.KindPrimary
)

func showDialog(cfg dialogConfig) func() {
	cfg.TopInset = preview.TopInset()
	cfg.BottomInset = preview.BottomInset()
	return decor.ShowDialog(cfg)
}
