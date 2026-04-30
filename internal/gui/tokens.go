package gui

import "image/color"

const (
	spaceXS  = 2
	spaceSM  = 4
	spaceMD  = 6
	spaceLG  = 8
	spaceXL  = 12
	spaceXXL = 16
)

const (
	textCaption = 10
	textBody    = 11
	textLabel   = 12
	textRow     = 13
	textHeading = 14
)

const (
	sidebarMaxWidth   = 300
	sidebarMinWidth   = 260
	sidebarStackBelow = 820
)

var (
	borderSubtle  color.Color = colGhostBorder
	borderDefault color.Color = colOutline
	borderStrong  color.Color = colDialogBorder
	borderAccent  color.Color = colPrimaryGhost
)

var (
	actionPrimary color.Color = colPrimary
	actionDanger  color.Color = colError
)

var (
	surfacePanel  color.Color = colSurfLowest
	surfaceRaised color.Color = colSurfLow
)
