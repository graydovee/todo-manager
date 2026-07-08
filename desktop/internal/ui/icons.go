package ui

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// IconKind identifies which vector glyph to draw.
type IconKind int

const (
	IconRefresh IconKind = iota
	IconPin
	IconUnpin
	IconLock
	IconUnlock
	IconSettings
	IconClose
	IconPlay
	IconCheck
	IconBack
)

// iconButtonSize is the square hit area for icon buttons.
const iconButtonSize = 30

// iconDesignSize is the side of the local coordinate box each glyph is drawn in.
const iconDesignSize = 16.0

// iconColor is the greyscale used for icon strokes.
var iconColor = color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF}

// iconButton lays out a clickable square containing a vector icon. The square
// highlights on hover/press via the Clickable state. The active flag inverts the
// fill to signal a toggled-on state (e.g. pinned/locked).
func iconButton(gtx layout.Context, th *material.Theme, c *widget.Clickable, kind IconKind, active bool) layout.Dimensions {
	// Compute the button pixel size up front from the dp value, independent of
	// whatever constraints the Clickable/Background layer propagates.
	btnPx := float32(gtx.Dp(unit.Dp(iconButtonSize)))
	sizePx := int(btnPx)
	gtx.Constraints.Min.X = sizePx
	gtx.Constraints.Max.X = sizePx
	gtx.Constraints.Min.Y = sizePx
	gtx.Constraints.Max.Y = sizePx

	bg := color.NRGBA{}
	fg := iconColor
	if active {
		bg = textPrimary
		fg = bgPage
	}

	return material.Clickable(gtx, c, func(gtx layout.Context) layout.Dimensions {
		// Fill background.
		if bg != (color.NRGBA{}) {
			fillRectSize(gtx, bg, sizePx)
		}
		// Draw the icon centred and scaled to ~64% of the button size.
		iconPx := btnPx * 0.64
		scale := iconPx / iconDesignSize
		offset := (btnPx - iconPx) / 2
		t := f32.AffineId().
			Scale(f32.Point{}, f32.Pt(scale, scale)).
			Offset(f32.Pt(offset, offset))
		defer op.Affine(t).Push(gtx.Ops).Pop()
		drawIcon(gtx.Ops, kind, fg)
		return layout.Dimensions{Size: image.Pt(sizePx, sizePx)}
	})
}

// iconButtonLabel is kept for callers that still render plain text (back buttons
// in detail/manage reuse it). It draws the icon kind with no toggle state.
func iconButtonSimple(gtx layout.Context, th *material.Theme, c *widget.Clickable, kind IconKind) layout.Dimensions {
	return iconButton(gtx, th, c, kind, false)
}

// fillRectSize paints an explicit pixel square with c (constraint-independent).
func fillRectSize(gtx layout.Context, c color.NRGBA, px int) {
	defer clip.Rect{Max: image.Pt(px, px)}.Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, c)
}

// drawIcon renders the given glyph in a 16x16 local coordinate box. Strokes use
// a 1.6-unit width. Circles/arcs are approximated by polylines for predictable,
// symmetric shapes (Gio's Arc params are fiddly).
func drawIcon(ops *op.Ops, kind IconKind, c color.NRGBA) {
	const sw = 1.6 // stroke width
	stroke := func(path clip.PathSpec) {
		defer clip.Stroke{Path: path, Width: sw}.Op().Push(ops).Pop()
		paint.ColorOp{Color: c}.Add(ops)
		paint.PaintOp{}.Add(ops)
	}
	fill := func(path clip.PathSpec) {
		defer clip.Outline{Path: path}.Op().Push(ops).Pop()
		paint.ColorOp{Color: c}.Add(ops)
		paint.PaintOp{}.Add(ops)
	}

	// circleStroke appends a polyline circle outline of the given radius centred
	// at (cx,cy) to a fresh path and strokes it.
	circleStroke := func(cx, cy, r float32) {
		var pc clip.Path
		pc.Begin(ops)
		polylineCircle(&pc, cx, cy, r, 0, math.Pi*2)
		stroke(pc.End())
	}

	switch kind {
	case IconRefresh:
		// A 3/4 open circle with an arrowhead at the top-right end.
		var pc clip.Path
		pc.Begin(ops)
		// Start the arc at top (12 o'clock) going clockwise ~300°.
		const cx, cy, r = 8, 8, 4.2
		start := float32(-math.Pi / 2) // top
		polylineCircle(&pc, cx, cy, r, start, start+float32(math.Pi)*1.65)
		stroke(pc.End())
		// Arrowhead chevron at the open end (top-right area).
		var ah clip.Path
		ah.Begin(ops)
		ah.MoveTo(f32.Pt(11.8, 2.2))
		ah.LineTo(f32.Pt(11.8, 5.6))
		ah.LineTo(f32.Pt(8.4, 5.6))
		stroke(ah.End())
	case IconPin:
		// Classic pushpin: teardrop head pointing down, with a shaft + tip.
		var pc clip.Path
		pc.Begin(ops)
		// Head: a downward teardrop (wide top, narrowing to shaft).
		pc.MoveTo(f32.Pt(8, 2.5))
		pc.LineTo(f32.Pt(11, 5.5))
		pc.LineTo(f32.Pt(8, 8.5))
		pc.LineTo(f32.Pt(5, 5.5))
		pc.Close()
		fill(pc.End())
		// Shaft + tip.
		var sh clip.Path
		sh.Begin(ops)
		sh.MoveTo(f32.Pt(8, 8.5))
		sh.LineTo(f32.Pt(8, 13.5))
		stroke(sh.End())
	case IconUnpin:
		// Same pin, outlined (not filled) to read as "inactive/off".
		var pc clip.Path
		pc.Begin(ops)
		pc.MoveTo(f32.Pt(8, 2.5))
		pc.LineTo(f32.Pt(11, 5.5))
		pc.LineTo(f32.Pt(8, 8.5))
		pc.LineTo(f32.Pt(5, 5.5))
		pc.Close()
		stroke(pc.End())
		var sh clip.Path
		sh.Begin(ops)
		sh.MoveTo(f32.Pt(8, 8.5))
		sh.LineTo(f32.Pt(8, 13.5))
		stroke(sh.End())
	case IconLock:
		// Padlock: filled rounded body + closed shackle arc on top.
		var body clip.Path
		body.Begin(ops)
		roundRect(&body, 3.5, 7, 9, 6, 1.4)
		fill(body.End())
		// Shackle: a half-ring above the body.
		var sh clip.Path
		sh.Begin(ops)
		polylineCircle(&sh, 8, 7, 2.4, math.Pi, 0) // upper half
		stroke(sh.End())
	case IconUnlock:
		// Padlock with an open shackle (raised right side).
		var body clip.Path
		body.Begin(ops)
		roundRect(&body, 3.5, 7, 9, 6, 1.4)
		fill(body.End())
		var sh clip.Path
		sh.Begin(ops)
		// Left side up + partial arc.
		sh.MoveTo(f32.Pt(5.6, 7))
		sh.LineTo(f32.Pt(5.6, 4.6))
		polylineCircle(&sh, 8, 4.6, 2.4, math.Pi, math.Pi*1.45)
		stroke(sh.End())
	case IconSettings:
		// Gear: two concentric circles (ring + hole).
		circleStroke(8, 8, 4.6)
		circleStroke(8, 8, 1.8)
		// Four small "teeth" ticks at N/E/S/W for a gear read.
		var t clip.Path
		t.Begin(ops)
		teeth := []f32.Point{
			f32.Pt(8, 1.4), f32.Pt(14.6, 8), f32.Pt(8, 14.6), f32.Pt(1.4, 8),
		}
		for _, pt := range teeth {
			t.MoveTo(pt)
			// short tick toward centre
			dx := (8 - pt.X) * 0.18
			dy := (8 - pt.Y) * 0.18
			t.LineTo(f32.Pt(pt.X+dx, pt.Y+dy))
		}
		stroke(t.End())
	case IconClose:
		// Bold X.
		var pc clip.Path
		pc.Begin(ops)
		pc.MoveTo(f32.Pt(4, 4))
		pc.LineTo(f32.Pt(12, 12))
		pc.MoveTo(f32.Pt(12, 4))
		pc.LineTo(f32.Pt(4, 12))
		stroke(pc.End())
	case IconPlay:
		// Filled triangle pointing right.
		var pc clip.Path
		pc.Begin(ops)
		pc.MoveTo(f32.Pt(5, 3))
		pc.LineTo(f32.Pt(13, 8))
		pc.LineTo(f32.Pt(5, 13))
		pc.Close()
		fill(pc.End())
	case IconCheck:
		// Check mark.
		var pc clip.Path
		pc.Begin(ops)
		pc.MoveTo(f32.Pt(3, 8.5))
		pc.LineTo(f32.Pt(6.5, 12))
		pc.LineTo(f32.Pt(13, 4.5))
		stroke(pc.End())
	case IconBack:
		// Left-pointing arrow.
		var pc clip.Path
		pc.Begin(ops)
		pc.MoveTo(f32.Pt(13, 8))
		pc.LineTo(f32.Pt(3, 8))
		pc.MoveTo(f32.Pt(7, 4))
		pc.LineTo(f32.Pt(3, 8))
		pc.LineTo(f32.Pt(7, 12))
		stroke(pc.End())
	}
}

// polylineCircle appends a circular arc (polyline approximation) to p, centred at
// (cx,cy) with radius r, from angle a0 to a1 (radians, counter-clockwise).
func polylineCircle(p *clip.Path, cx, cy, r, a0, a1 float32) {
	const seg = 48.0
	steps := int(math.Ceil(math.Abs(float64(a1-a0)) / (2 * math.Pi) * seg))
	if steps < 2 {
		steps = 2
	}
	for i := 0; i <= steps; i++ {
		a := a0 + (a1-a0)*float32(i)/float32(steps)
		x := cx + r*float32(math.Cos(float64(a)))
		y := cy + r*float32(math.Sin(float64(a)))
		if i == 0 {
			p.MoveTo(f32.Pt(x, y))
		} else {
			p.LineTo(f32.Pt(x, y))
		}
	}
}

// roundRect appends a rounded rectangle (radius rr at every corner) to p as a
// closed contour, starting a fresh sub-path via MoveTo.
func roundRect(p *clip.Path, x, y, w, h, rr float32) {
	// Clamp radius.
	if rr > w/2 {
		rr = w / 2
	}
	if rr > h/2 {
		rr = h / 2
	}
	p.MoveTo(f32.Pt(x+rr, y))
	p.LineTo(f32.Pt(x+w-rr, y))
	arcCorner(p, x+w-rr, y+rr, rr, -math.Pi/2, 0)
	p.LineTo(f32.Pt(x+w, y+h-rr))
	arcCorner(p, x+w-rr, y+h-rr, rr, 0, math.Pi/2)
	p.LineTo(f32.Pt(x+rr, y+h))
	arcCorner(p, x+rr, y+h-rr, rr, math.Pi/2, math.Pi)
	p.LineTo(f32.Pt(x, y+rr))
	arcCorner(p, x+rr, y+rr, rr, math.Pi, math.Pi*1.5)
	p.Close()
}

// arcCorner appends a quarter-circle polyline at corner (cx,cy).
func arcCorner(p *clip.Path, cx, cy, r, a0, a1 float32) {
	polylineCircle(p, cx, cy, r, a0, a1)
}

// smallIconButton lays out a labelled action button that pairs an icon with a
// short text label (e.g. "Start" with a play glyph).
func smallIconButton(gtx layout.Context, th *material.Theme, c *widget.Clickable, kind IconKind, label string) layout.Dimensions {
	btn := material.Button(th, c, label)
	btn.TextSize = unit.Sp(11)
	btn.Inset = layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(3), Left: unit.Dp(6), Right: unit.Dp(6)}
	btn.Background = textPrimary
	btn.Color = bgPage
	return btn.Layout(gtx)
}
