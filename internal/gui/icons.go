package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// SVG cleaned for Fyne's themed resource recolor: named "black" colors,
// explicit stroke-width, no fixed pixel width/height (viewBox governs scale).
var clockSVG = []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 21 21"><g fill="none" stroke="black" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" transform="matrix(-1 0 0 1 19 2)"><circle cx="8.5" cy="8.5" r="8"/><path d="M8.5 5.5v4H5"/></g></svg>`)

var clockIconResource fyne.Resource = theme.NewThemedResource(fyne.NewStaticResource("clock.svg", clockSVG))
