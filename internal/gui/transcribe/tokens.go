package transcribe

import (
	"github.com/asolopovas/wt/internal/gui/decor"
	"github.com/asolopovas/wt/internal/gui/preview"
)

const (
	spaceMD  = decor.SpaceMD
	spaceLG  = decor.SpaceLG
	spaceXL  = decor.SpaceXL
	spaceXXL = decor.SpaceXXL
)

const (
	textCaption = decor.TextCaption
	textBody    = decor.TextBody
)

var surfaceRaised = decor.SurfaceRaised

var (
	colMuted        = decor.TextMuted
	colPrimary      = decor.ActionPrimary
	colPrimaryFaint = decor.ActionPrimaryFaint
	transparent     = decor.Transparent
)

var monoBoldStyle = decor.MonoBoldStyle

var (
	newPrimaryButton         = decor.NewPrimaryButton
	newSecondaryButton       = decor.NewSecondaryButton
	wrapAction               = decor.WrapAction
	newCaptionText           = decor.NewCaptionText
	newPanelBackground       = decor.NewPanelBackground
	newPointerButton         = decor.NewPointerButton
	newPointerButtonWithIcon = decor.NewPointerButtonWithIcon
	newThinProgress          = decor.NewThinProgress
)

type (
	pointerButton = decor.PointerButton
	thinProgress  = decor.ThinProgress
)

type (
	dialogAction = decor.DialogAction
	dialogConfig = decor.DialogConfig
)

const (
	kindSecondary = decor.KindSecondary
	kindPrimary   = decor.KindPrimary
)

const (
	notifyInfo   = decor.NotifyInfo
	notifyActive = decor.NotifyActive
)

var (
	setStatusText  = decor.SetStatusText
	setStatusStyle = decor.SetStatusStyle
	showNotice     = decor.ShowNotice
	showError      = decor.ShowError
)

func showDialog(cfg dialogConfig) func() {
	cfg.TopInset = preview.TopInset()
	cfg.BottomInset = preview.BottomInset()
	return decor.ShowDialog(cfg)
}
