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
	IconPlus
)

// iconButtonSize is the square hit area for icon buttons.
const iconButtonSize = 30

// iconDesignSize is the side of the local coordinate box each glyph is drawn in.
const iconDesignSize = 16.0

// strokeWidth is the unified stroke width for all line-art icons (in design
// units). 1.5 gives a crisp, modern Feather/Lucide-style line at the icon's
// rendered scale.
const strokeWidth = 1.5

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

// iconButtonSimple draws an icon button with no toggle state.
func iconButtonSimple(gtx layout.Context, th *material.Theme, c *widget.Clickable, kind IconKind) layout.Dimensions {
	return iconButton(gtx, th, c, kind, false)
}

// fillRectSize paints an explicit pixel square with c (constraint-independent).
func fillRectSize(gtx layout.Context, c color.NRGBA, px int) {
	defer clip.Rect{Max: image.Pt(px, px)}.Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, c)
}

// -----------------------------------------------------------------------
// Drawing primitives
// -----------------------------------------------------------------------

// strokePath strokes a completed PathSpec with the icon colour.
func strokePath(ops *op.Ops, spec clip.PathSpec) {
	defer clip.Stroke{Path: spec, Width: strokeWidth}.Op().Push(ops).Pop()
	paint.ColorOp{Color: iconColor}.Add(ops)
	paint.PaintOp{}.Add(ops)
}

// strokePathCol strokes a completed PathSpec with a specific colour.
func strokePathCol(ops *op.Ops, spec clip.PathSpec, c color.NRGBA) {
	defer clip.Stroke{Path: spec, Width: strokeWidth}.Op().Push(ops).Pop()
	paint.ColorOp{Color: c}.Add(ops)
	paint.PaintOp{}.Add(ops)
}

// fillPath fills the interior of a completed PathSpec with a specific colour.
func fillPath(ops *op.Ops, spec clip.PathSpec, c color.NRGBA) {
	defer clip.Outline{Path: spec}.Op().Push(ops).Pop()
	paint.ColorOp{Color: c}.Add(ops)
	paint.PaintOp{}.Add(ops)
}

// arcTo appends a circular arc to the path. cx/cy is the centre, r the radius,
// a0 the start angle and sweep the angular extent (both in radians). Positive
// sweep goes counter-clockwise in screen space (y-down).
func arcTo(p *clip.Path, cx, cy, r, a0, sweep float32) {
	// Move pen to the arc start.
	sx := cx + r*float32(math.Cos(float64(a0)))
	sy := cy + r*float32(math.Sin(float64(a0)))
	p.MoveTo(f32.Pt(sx, sy))
	// ArcTo with coincident foci draws a circular arc.
	p.ArcTo(f32.Pt(cx, cy), f32.Pt(cx, cy), sweep)
}

// roundRect appends a rounded rectangle to p as a closed contour, using cubic
// Bézier corners (the standard circle approximation, same formula as Gio's own
// clip.RRect). This avoids ArcTo angle-sign pitfalls entirely.
func roundRect(p *clip.Path, ops *op.Ops, x, y, w, h, rr float32) {
	if rr > w/2 {
		rr = w / 2
	}
	if rr > h/2 {
		rr = h / 2
	}
	// q is the standard Bézier circle-approximation constant.
	const q = 4 * (math.Sqrt2 - 1) / 3
	iq := rr * (1 - q)

	// Start at top-left corner end of the top edge.
	p.MoveTo(f32.Pt(x+rr, y))
	// Top edge → NE corner.
	p.LineTo(f32.Pt(x+w-rr, y))
	p.CubeTo(
		f32.Pt(x+w-rr*iq, y),
		f32.Pt(x+w, y+rr*iq),
		f32.Pt(x+w, y+rr),
	)
	// Right edge → SE corner.
	p.LineTo(f32.Pt(x+w, y+h-rr))
	p.CubeTo(
		f32.Pt(x+w, y+h-rr*iq),
		f32.Pt(x+w-rr*iq, y+h),
		f32.Pt(x+w-rr, y+h),
	)
	// Bottom edge → SW corner.
	p.LineTo(f32.Pt(x+rr, y+h))
	p.CubeTo(
		f32.Pt(x+rr*iq, y+h),
		f32.Pt(x, y+h-rr*iq),
		f32.Pt(x, y+h-rr),
	)
	// Left edge → NW corner.
	p.LineTo(f32.Pt(x, y+rr))
	p.CubeTo(
		f32.Pt(x, y+rr*iq),
		f32.Pt(x+rr*iq, y),
		f32.Pt(x+rr, y),
	)
	p.Close()
}

// -----------------------------------------------------------------------
// Icons — drawn in a 16×16 design grid, centred at (8,8).
// -----------------------------------------------------------------------

// drawIcon renders the given glyph using the supplied colour.
func drawIcon(ops *op.Ops, kind IconKind, c color.NRGBA) {
	switch kind {
	case IconRefresh:
		drawRefresh(ops, c)
	case IconPin:
		drawPin(ops, c, false)
	case IconUnpin:
		drawPin(ops, c, true)
	case IconLock:
		drawLock(ops, c, false)
	case IconUnlock:
		drawLock(ops, c, true)
	case IconSettings:
		drawSettings(ops, c)
	case IconClose:
		drawClose(ops, c)
	case IconPlay:
		drawPlay(ops, c)
	case IconCheck:
		drawCheck(ops, c)
	case IconBack:
		drawBack(ops, c)
	case IconPlus:
		drawPlus(ops, c)
	}
}

// drawRefresh: a clockwise circular arrow. The arc spans ~270° starting from
// the upper-right and the arrowhead sits exactly at the arc's end. Built as a
// single polyline path so the arrowhead endpoint is guaranteed to match the arc.
func drawRefresh(ops *op.Ops, c color.NRGBA) {
	const (
		cx = 8.0
		cy = 8.0
		r  = 4.5
	)
	// The arc goes clockwise from the top (-90°) around to ~+200° (lower-left).
	// In y-down screen coords, increasing angle = clockwise.
	const (
		aStart = float32(-math.Pi / 2)              // top
		aEnd   = float32(math.Pi * 1.1)             // ~198° (lower-left area)
	)
	const segs = 64

	// Build the arc polyline.
	var pc clip.Path
	pc.Begin(ops)
	for i := 0; i <= segs; i++ {
		t := float32(i) / float32(segs)
		a := aStart + (aEnd-aStart)*t
		pt := f32.Pt(cx+r*float32(math.Cos(float64(a))), cy+r*float32(math.Sin(float64(a))))
		if i == 0 {
			pc.MoveTo(pt)
		} else {
			pc.LineTo(pt)
		}
	}

	// Arrowhead at the END of the arc. The tip is the last arc point.
	// Tangent direction = derivative w.r.t. angle, which for clockwise travel
	// (increasing angle) is (-sin(a), cos(a)).
	tipX := cx + r*float32(math.Cos(float64(aEnd)))
	tipY := cy + r*float32(math.Sin(float64(aEnd)))
	tangX := -float32(math.Sin(float64(aEnd)))
	tangY := float32(math.Cos(float64(aEnd)))
	// Normal (perpendicular).
	normX := -tangY
	normY := tangX

	// Arrowhead: two arms going backward from the tip.
	const (
		armLen  = 2.5
		splayAng = 0.5 // radians, ~28° each side
	)
	// Left arm: rotate tangent backward by +splayAng.
	c1 := float32(math.Cos(float64(splayAng)))
	s1 := float32(math.Sin(float64(splayAng)))
	// Backward tangent.
	btx, bty := -tangX, -tangY
	// Rotate backward tangent by ±splayAng.
	for _, sign := range []float32{1, -1} {
		rx := btx*c1 - bty*s1*sign
		ry := bty*c1 + btx*s1*sign
		armX := tipX + armLen*rx
		armY := tipY + armLen*ry
		pc.MoveTo(f32.Pt(tipX, tipY))
		pc.LineTo(f32.Pt(armX, armY))
	}
	_ = normX
	_ = normY
	strokePathCol(ops, pc.End(), c)
}

// drawPin: a classic thumbtack/pushpin — the icon for "pin to top". It has a
// round flat head at the top and a pointed shaft below. Drawn with explicit
// bezier control points for a clean, symmetric shape. When slashed, a diagonal
// line crosses it (unpin).
func drawPin(ops *op.Ops, c color.NRGBA, slashed bool) {
	// The pin head is a horizontal capsule (rounded rectangle) at the top, and
	// the shaft is a triangle tapering to a point below.
	//
	//   Head:  rounded bar from (5,3) to (11,3), 2.5 tall, rounded ends
	//   Shaft: triangle from the head bottom narrowing to tip at (8,14)

	// Head as a rounded horizontal bar (capsule).
	var head clip.Path
	head.Begin(ops)
	roundRect(&head, ops, 4.8, 2.8, 6.4, 2.6, 1.3)
	strokePathCol(ops, head.End(), c)

	// Shaft: two lines from the head bottom converging to the tip.
	var shaft clip.Path
	shaft.Begin(ops)
	shaft.MoveTo(f32.Pt(6.2, 5.6))
	shaft.LineTo(f32.Pt(8, 14))
	shaft.LineTo(f32.Pt(9.8, 5.6))
	strokePathCol(ops, shaft.End(), c)

	// A small crossbar where the shaft meets the head.
	var bar clip.Path
	bar.Begin(ops)
	bar.MoveTo(f32.Pt(6, 5.4))
	bar.LineTo(f32.Pt(10, 5.4))
	strokePathCol(ops, bar.End(), c)

	if slashed {
		var slash clip.Path
		slash.Begin(ops)
		slash.MoveTo(f32.Pt(3, 3))
		slash.LineTo(f32.Pt(13, 13))
		strokePathCol(ops, slash.End(), c)
	}
}

// drawLock: a padlock — rounded body + shackle arc. When open is true the
// shackle is broken (right side lifted), indicating unlocked.
func drawLock(ops *op.Ops, c color.NRGBA, open bool) {
	// Body: rounded rectangle.
	var body clip.Path
	body.Begin(ops)
	roundRect(&body, ops, 4, 7.5, 8, 5.5, 1.3)
	strokePathCol(ops, body.End(), c)

	// Keyhole dot.
	const q = 4 * (math.Sqrt2 - 1) / 3
	var dot clip.Path
	dot.Begin(ops)
	drawCircleBezier(&dot, ops, 8, 10.2, 0.85, q)
	dot.Close()
	fillPath(ops, dot.End(), c)

	// Shackle: a half-ring above the body, drawn as a semicircle via cubic
	// Bézier (avoids ArcTo angle-sign ambiguity). The shackle spans from
	// (sx-sr, top) over the top to (sx+sr, top), where top is the apex.
	const (
		sx  float32 = 8.0
		sy  float32 = 7.5 // shackle base (sits on the lock body top edge)
		sr  float32 = 2.3 // shackle radius
	)
	iqv := sr * (1 - q)
	top := sy - sr           // y-coordinate of the horizontal centre line of the arc
	arcApexY := top - sr     // topmost point of the shackle arc

	if open {
		// Left leg up + partial arc ending early (right side lifted free).
		var sh clip.Path
		sh.Begin(ops)
		sh.MoveTo(f32.Pt(sx-sr, sy))
		sh.LineTo(f32.Pt(sx-sr, top))
		// Left quarter: (sx-sr, top) → (sx, arcApexY).
		sh.CubeTo(
			f32.Pt(sx-sr, top-iqv),
			f32.Pt(sx-sr+iqv, arcApexY),
			f32.Pt(sx, arcApexY),
		)
		// Right quarter: (sx, arcApexY) → (sx+sr, top).
		sh.CubeTo(
			f32.Pt(sx+iqv, arcApexY),
			f32.Pt(sx+sr, top-iqv),
			f32.Pt(sx+sr, top),
		)
		// Right side lifted up and away (unlocked = open shackle).
		sh.LineTo(f32.Pt(sx+sr, top-1.5))
		strokePathCol(ops, sh.End(), c)
	} else {
		// Full closed shackle: left leg up, half-circle over, right leg down.
		var sh clip.Path
		sh.Begin(ops)
		sh.MoveTo(f32.Pt(sx-sr, sy))
		sh.LineTo(f32.Pt(sx-sr, top))
		// Left quarter: (sx-sr, top) → (sx, arcApexY).
		sh.CubeTo(
			f32.Pt(sx-sr, top-iqv),
			f32.Pt(sx-sr+iqv, arcApexY),
			f32.Pt(sx, arcApexY),
		)
		// Right quarter: (sx, arcApexY) → (sx+sr, top).
		sh.CubeTo(
			f32.Pt(sx+iqv, arcApexY),
			f32.Pt(sx+sr, top-iqv),
			f32.Pt(sx+sr, top),
		)
		sh.LineTo(f32.Pt(sx+sr, sy))
		strokePathCol(ops, sh.End(), c)
	}
}

// drawCircleBezier appends a full circle to p using 4 cubic Béziers (the
// standard circle approximation with q = 4*(√2-1)/3).
func drawCircleBezier(p *clip.Path, ops *op.Ops, cx, cy, r, q float32) {
	// q is the tangent handle fraction; the inner handle is at (1-q)*r from
	// the axis. Start at the top point (cx, cy-r), go clockwise.
	p.MoveTo(f32.Pt(cx, cy-r))
	// Top → right quarter.
	p.CubeTo(f32.Pt(cx+r*q, cy-r), f32.Pt(cx+r, cy-r*q), f32.Pt(cx+r, cy))
	// Right → bottom quarter.
	p.CubeTo(f32.Pt(cx+r, cy+r*q), f32.Pt(cx+r*q, cy+r), f32.Pt(cx, cy+r))
	// Bottom → left quarter.
	p.CubeTo(f32.Pt(cx-r*q, cy+r), f32.Pt(cx-r, cy+r*q), f32.Pt(cx-r, cy))
	// Left → top quarter.
	p.CubeTo(f32.Pt(cx-r, cy-r*q), f32.Pt(cx-r*q, cy-r), f32.Pt(cx, cy-r))
}

// drawSettings: three horizontal sliders with circular handles at different
// positions — the modern Feather/Lucide-style settings icon. Clean and
// unambiguous at small sizes (a gear rendered at 16px often looks like a
// flower/sun, whereas sliders are instantly readable).
func drawSettings(ops *op.Ops, c color.NRGBA) {
	const (
		lineY1, dotX1 = 4.0, 11.0  // top slider: handle on the right
		lineY2, dotX2 = 8.0, 5.0   // middle slider: handle on the left
		lineY3, dotX3 = 12.0, 9.0  // bottom slider: handle on the right
	)
	const dotR = 1.5
	const q = 4 * (math.Sqrt2 - 1) / 3

	// Draw a slider: a horizontal line from x=3 to x=13, with a filled circle
	// (handle) at dotX.
	slider := func(y, dotX float32) {
		var line clip.Path
		line.Begin(ops)
		line.MoveTo(f32.Pt(3, y))
		line.LineTo(f32.Pt(13, y))
		strokePathCol(ops, line.End(), c)

		var dot clip.Path
		dot.Begin(ops)
		drawCircleBezier(&dot, ops, dotX, y, dotR, q)
		dot.Close()
		fillPath(ops, dot.End(), c)
		// Re-stroke the line segment hidden behind the dot for a clean look.
		var over clip.Path
		over.Begin(ops)
		over.MoveTo(f32.Pt(dotX-dotR, y))
		over.LineTo(f32.Pt(dotX+dotR, y))
		strokePathCol(ops, over.End(), c)
	}

	slider(lineY1, dotX1)
	slider(lineY2, dotX2)
	slider(lineY3, dotX3)
}

// drawClose: a bold X — two diagonal strokes.
func drawClose(ops *op.Ops, c color.NRGBA) {
	var pc clip.Path
	pc.Begin(ops)
	pc.MoveTo(f32.Pt(4.5, 4.5))
	pc.LineTo(f32.Pt(11.5, 11.5))
	pc.MoveTo(f32.Pt(11.5, 4.5))
	pc.LineTo(f32.Pt(4.5, 11.5))
	strokePathCol(ops, pc.End(), c)
}

// drawPlay: a filled right-pointing triangle with slightly rounded corners.
func drawPlay(ops *op.Ops, c color.NRGBA) {
	var pc clip.Path
	pc.Begin(ops)
	pc.MoveTo(f32.Pt(5.5, 3.5))
	pc.LineTo(f32.Pt(12.5, 8))
	pc.LineTo(f32.Pt(5.5, 12.5))
	pc.Close()
	fillPath(ops, pc.End(), c)
}

// drawCheck: a check mark with a rounded corner at the elbow.
func drawCheck(ops *op.Ops, c color.NRGBA) {
	var pc clip.Path
	pc.Begin(ops)
	pc.MoveTo(f32.Pt(3, 8))
	// Straight line to near the elbow.
	pc.LineTo(f32.Pt(6.2, 11))
	// Rounded transition then up to the top-right tip.
	pc.QuadTo(f32.Pt(7.0, 10.0), f32.Pt(7.5, 9.2))
	pc.LineTo(f32.Pt(13, 4.5))
	strokePathCol(ops, pc.End(), c)
}

// drawBack: a left-pointing arrow — horizontal line + chevron head.
func drawBack(ops *op.Ops, c color.NRGBA) {
	var pc clip.Path
	pc.Begin(ops)
	// Chevron head.
	pc.MoveTo(f32.Pt(7, 4))
	pc.LineTo(f32.Pt(4, 8))
	pc.LineTo(f32.Pt(7, 12))
	strokePathCol(ops, pc.End(), c)

	// Shaft.
	var sh clip.Path
	sh.Begin(ops)
	sh.MoveTo(f32.Pt(4, 8))
	sh.LineTo(f32.Pt(13, 8))
	strokePathCol(ops, sh.End(), c)
}

// drawPlus: a plus sign — horizontal + vertical line crossing at centre.
func drawPlus(ops *op.Ops, c color.NRGBA) {
	var pc clip.Path
	pc.Begin(ops)
	// Horizontal line.
	pc.MoveTo(f32.Pt(4, 8))
	pc.LineTo(f32.Pt(12, 8))
	// Vertical line.
	pc.MoveTo(f32.Pt(8, 4))
	pc.LineTo(f32.Pt(8, 12))
	strokePathCol(ops, pc.End(), c)
}
