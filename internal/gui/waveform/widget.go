//go:build !android

package waveform

import (
	"image"
	"image/color"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

const (
	handleWidth      = 8
	minRegionFrac    = 0.005
	timeLabelMargin  = 4
	timeLabelMinSize = 9
)

var (
	colWaveform   = color.NRGBA{R: 220, G: 222, B: 226, A: 255}
	colWaveformBG = color.NRGBA{R: 22, G: 26, B: 32, A: 255}
	colHandle     = color.NRGBA{R: 235, G: 145, B: 60, A: 230}
	colRegion     = color.NRGBA{R: 235, G: 145, B: 60, A: 32}
	colPlayhead   = color.NRGBA{R: 255, G: 255, B: 255, A: 220}
	colTimeLabel = color.NRGBA{R: 235, G: 235, B: 235, A: 255}
)

// Widget is a horizontal waveform strip with two draggable region handles
// and a playhead. Region values are normalized [0,1].
type Widget struct {
	widget.BaseWidget

	mu    sync.Mutex
	peaks *Peaks
	loading bool

	regionStart float64 // 0..1
	regionEnd   float64 // 0..1
	playhead    float64 // 0..1, -1 hides

	// callbacks (called from UI thread)
	OnRegionChanged func(start, end float64)
	OnSeek          func(pos float64) // tap on waveform body

	// drag state
	dragKind  dragKind
	dragStart float64
}

type dragKind int

const (
	dragNone dragKind = iota
	dragStartHandle
	dragEndHandle
	dragRegion
)

func New() *Widget {
	w := &Widget{regionStart: 0, regionEnd: 1, playhead: -1}
	w.ExtendBaseWidget(w)
	return w
}

func (w *Widget) SetPeaks(p *Peaks) {
	w.mu.Lock()
	w.peaks = p
	w.loading = false
	w.mu.Unlock()
	w.Refresh()
}

func (w *Widget) SetLoading(b bool) {
	w.mu.Lock()
	w.loading = b
	if b {
		w.peaks = nil
	}
	w.mu.Unlock()
	w.Refresh()
}

func (w *Widget) Region() (float64, float64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.regionStart, w.regionEnd
}

func (w *Widget) SetRegion(start, end float64) {
	if start < 0 {
		start = 0
	}
	if end > 1 {
		end = 1
	}
	if end < start+minRegionFrac {
		end = start + minRegionFrac
		if end > 1 {
			end = 1
			start = end - minRegionFrac
		}
	}
	w.mu.Lock()
	w.regionStart, w.regionEnd = start, end
	cb := w.OnRegionChanged
	w.mu.Unlock()
	if cb != nil {
		cb(start, end)
	}
	w.Refresh()
}

func (w *Widget) SetPlayhead(pos float64) {
	w.mu.Lock()
	w.playhead = pos
	w.mu.Unlock()
	w.Refresh()
}

func (w *Widget) Duration() float64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.peaks == nil {
		return 0
	}
	return w.peaks.Duration
}

func (w *Widget) MinSize() fyne.Size {
	return fyne.NewSize(200, 64)
}

// --- Renderer ---

type waveformRenderer struct {
	w        *Widget
	bg       *canvas.Rectangle
	wave     *canvas.Image
	region   *canvas.Rectangle
	startH   *canvas.Rectangle
	endH     *canvas.Rectangle
	playhead *canvas.Line
	leftLbl  *canvas.Text
	rightLbl *canvas.Text
	spinner  *widget.Activity
	objects  []fyne.CanvasObject

	lastSize fyne.Size
}

func (w *Widget) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(colWaveformBG)
	wave := canvas.NewImageFromImage(blankImage(2, 2))
	wave.FillMode = canvas.ImageFillStretch
	wave.ScaleMode = canvas.ImageScalePixels
	region := canvas.NewRectangle(colRegion)
	startH := canvas.NewRectangle(colHandle)
	endH := canvas.NewRectangle(colHandle)
	playhead := canvas.NewLine(colPlayhead)
	playhead.StrokeWidth = 1.5
	leftLbl := canvas.NewText("0:00", colTimeLabel)
	leftLbl.TextSize = timeLabelMinSize
	leftLbl.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	rightLbl := canvas.NewText("0:00", colTimeLabel)
	rightLbl.TextSize = timeLabelMinSize
	rightLbl.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	spinner := widget.NewActivity()
	spinner.Hide()

	r := &waveformRenderer{
		w: w, bg: bg, wave: wave, region: region,
		startH: startH, endH: endH, playhead: playhead,
		leftLbl: leftLbl, rightLbl: rightLbl, spinner: spinner,
	}
	r.objects = []fyne.CanvasObject{bg, wave, region, startH, endH, playhead, leftLbl, rightLbl, spinner}
	return r
}

func (r *waveformRenderer) Layout(sz fyne.Size) {
	r.bg.Resize(sz)
	r.bg.Move(fyne.NewPos(0, 0))
	r.wave.Resize(sz)
	r.wave.Move(fyne.NewPos(0, 0))

	w := r.w
	w.mu.Lock()
	rs, re := w.regionStart, w.regionEnd
	ph := w.playhead
	loading := w.loading
	peaks := w.peaks
	w.mu.Unlock()

	// rasterize wave if peaks present and size changed
	if peaks != nil && (sz != r.lastSize) {
		r.wave.Image = renderPeaks(peaks, int(sz.Width), int(sz.Height))
		r.wave.Refresh()
		r.lastSize = sz
	}
	if peaks == nil {
		r.wave.Image = blankImage(int(sz.Width), int(sz.Height))
		r.wave.Refresh()
	}

	xS := float32(rs) * sz.Width
	xE := float32(re) * sz.Width
	r.region.Move(fyne.NewPos(xS, 0))
	r.region.Resize(fyne.NewSize(xE-xS, sz.Height))

	r.startH.Move(fyne.NewPos(xS-handleWidth/2, 0))
	r.startH.Resize(fyne.NewSize(handleWidth, sz.Height))
	r.endH.Move(fyne.NewPos(xE-handleWidth/2, 0))
	r.endH.Resize(fyne.NewSize(handleWidth, sz.Height))

	if ph >= 0 && ph <= 1 {
		x := float32(ph) * sz.Width
		r.playhead.Position1 = fyne.NewPos(x, 0)
		r.playhead.Position2 = fyne.NewPos(x, sz.Height)
		r.playhead.Show()
	} else {
		r.playhead.Hide()
	}

	dur := 0.0
	if peaks != nil {
		dur = peaks.Duration
	}
	r.leftLbl.Text = formatMS(rs * dur)
	r.rightLbl.Text = formatMS(re * dur)
	r.leftLbl.Refresh()
	r.rightLbl.Refresh()

	lblSize := r.leftLbl.MinSize()
	r.leftLbl.Move(fyne.NewPos(timeLabelMargin, sz.Height-lblSize.Height-timeLabelMargin))
	rSize := r.rightLbl.MinSize()
	r.rightLbl.Move(fyne.NewPos(sz.Width-rSize.Width-timeLabelMargin, sz.Height-rSize.Height-timeLabelMargin))

	if loading {
		r.spinner.Show()
		r.spinner.Start()
		spSize := fyne.NewSize(28, 28)
		r.spinner.Resize(spSize)
		r.spinner.Move(fyne.NewPos((sz.Width-spSize.Width)/2, (sz.Height-spSize.Height)/2))
	} else {
		r.spinner.Stop()
		r.spinner.Hide()
	}
}

func (r *waveformRenderer) MinSize() fyne.Size       { return r.w.MinSize() }
func (r *waveformRenderer) Refresh()                 { r.Layout(r.w.Size()) }
func (r *waveformRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *waveformRenderer) Destroy()                 {}

// --- Mouse / drag ---

var _ fyne.Draggable = (*Widget)(nil)
var _ fyne.Tappable = (*Widget)(nil)
var _ desktop.Cursorable = (*Widget)(nil)

func (w *Widget) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (w *Widget) Tapped(e *fyne.PointEvent) {
	frac := w.posFrac(e.Position.X)
	w.mu.Lock()
	rs, re := w.regionStart, w.regionEnd
	cb := w.OnSeek
	w.mu.Unlock()
	// If the tap is inside the region body, seek; outside, ignore (drag handles instead).
	if frac >= rs && frac <= re && cb != nil {
		cb(frac)
	}
}

func (w *Widget) Dragged(e *fyne.DragEvent) {
	frac := w.posFrac(e.Position.X)
	w.mu.Lock()
	if w.dragKind == dragNone {
		// classify based on starting position
		w.dragStart = frac
		switch {
		case nearFrac(frac, w.regionStart, w.handleFrac()):
			w.dragKind = dragStartHandle
		case nearFrac(frac, w.regionEnd, w.handleFrac()):
			w.dragKind = dragEndHandle
		case frac > w.regionStart && frac < w.regionEnd:
			w.dragKind = dragRegion
		default:
			// clicking outside region: move nearest handle to here
			if frac < w.regionStart {
				w.dragKind = dragStartHandle
			} else {
				w.dragKind = dragEndHandle
			}
		}
	}
	rs, re := w.regionStart, w.regionEnd
	switch w.dragKind {
	case dragStartHandle:
		rs = frac
	case dragEndHandle:
		re = frac
	case dragRegion:
		// move both by delta from previous frame
		dx := frac - w.dragStart
		w.dragStart = frac
		rs += dx
		re += dx
		if rs < 0 {
			re -= rs
			rs = 0
		}
		if re > 1 {
			rs -= re - 1
			re = 1
		}
	}
	if rs < 0 {
		rs = 0
	}
	if re > 1 {
		re = 1
	}
	if re < rs+minRegionFrac {
		if w.dragKind == dragStartHandle {
			rs = re - minRegionFrac
		} else {
			re = rs + minRegionFrac
		}
	}
	w.regionStart, w.regionEnd = rs, re
	cb := w.OnRegionChanged
	w.mu.Unlock()
	if cb != nil {
		cb(rs, re)
	}
	w.Refresh()
}

func (w *Widget) DragEnd() {
	w.mu.Lock()
	w.dragKind = dragNone
	w.mu.Unlock()
}

func (w *Widget) handleFrac() float64 {
	sz := w.Size()
	if sz.Width <= 0 {
		return 0.01
	}
	return float64(handleWidth*1.5) / float64(sz.Width)
}

func (w *Widget) posFrac(x float32) float64 {
	sz := w.Size()
	if sz.Width <= 0 {
		return 0
	}
	f := float64(x) / float64(sz.Width)
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	return f
}

func nearFrac(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

// --- Drawing ---

func renderPeaks(p *Peaks, w, h int) image.Image {
	if w <= 0 || h <= 0 {
		return blankImage(2, 2)
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	// fill bg
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, colWaveformBG)
		}
	}
	if len(p.Max) == 0 {
		return img
	}
	mid := h / 2
	for x := 0; x < w; x++ {
		idx := int(float64(x) * float64(len(p.Max)) / float64(w))
		if idx >= len(p.Max) {
			idx = len(p.Max) - 1
		}
		hi := p.Max[idx]
		lo := p.Min[idx]
		topY := mid - int(float32(mid)*hi)
		botY := mid - int(float32(mid)*lo)
		if topY > botY {
			topY, botY = botY, topY
		}
		if topY == botY {
			botY = topY + 1
		}
		if topY < 0 {
			topY = 0
		}
		if botY >= h {
			botY = h - 1
		}
		for y := topY; y <= botY; y++ {
			img.Set(x, y, colWaveform)
		}
	}
	return img
}

func blankImage(w, h int) image.Image {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, colWaveformBG)
		}
	}
	return img
}

func formatMS(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	d := time.Duration(sec * float64(time.Second))
	m := int(d / time.Minute)
	s := int((d % time.Minute) / time.Second)
	if m >= 60 {
		h := m / 60
		m = m % 60
		return formatN(h) + ":" + pad2(m) + ":" + pad2(s)
	}
	return formatN(m) + ":" + pad2(s)
}

func formatN(n int) string {
	if n < 10 {
		return string(rune('0'+n))
	}
	// fall back
	return itoa(n)
}

func pad2(n int) string {
	if n < 0 {
		n = 0
	}
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
