package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// separator draws a 1dp horizontal line in the soft border colour.
func separator(gtx layout.Context) layout.Dimensions {
	rect := image.Rect(0, 0, gtx.Constraints.Max.X, 1)
	defer clip.Rect(rect).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, borderSoft)
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 1)}
}

// centeredText lays out a label centred in the available area.
func centeredText(gtx layout.Context, th *material.Theme, msg string, c color.NRGBA) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		lbl := material.Body1(th, msg)
		lbl.Color = c
		return lbl.Layout(gtx)
	})
}

// colWidth constrains a column to a fixed dp width and renders body inside.
// It always reports the exact fixed width so header and data rows align even
// when their content differs in natural size.
func colWidth(gtx layout.Context, width unit.Dp, body layout.Widget) layout.Dimensions {
	px := gtx.Dp(width)
	gtx.Constraints.Max.X = px
	gtx.Constraints.Min.X = px
	dims := body(gtx)
	dims.Size.X = px
	return dims
}

// verticalCenter wraps body so it is vertically centred in the given area.
func verticalCenter(gtx layout.Context, body layout.Widget) layout.Dimensions {
	return layout.Center.Layout(gtx, body)
}

// insetRow pads a row's content.
func insetRow(gtx layout.Context, body layout.Widget) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(7), Left: unit.Dp(10), Right: unit.Dp(6)}.Layout(gtx, body)
}
