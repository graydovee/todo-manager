package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// buttonInset is the uniform padding for all text buttons.
var buttonInset = layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(12), Right: unit.Dp(12)}

// buttonFontSize is the unified text size for all buttons.
const buttonFontSize = unit.Sp(12)

// buttonMinHeight forces every button to the same height regardless of font
// metrics differences between CJK and Latin glyphs.
const buttonMinHeight = unit.Dp(32)

// styleButton applies the app-wide uniform button style. Used by smallButton
// and choiceButton.
func styleButton(btn *material.ButtonStyle) {
	btn.TextSize = buttonFontSize
	btn.Inset = buttonInset
	btn.CornerRadius = unit.Dp(4)
	btn.Background = textPrimary
	btn.Color = bgPage
}

// styleChipButton styles a toggle chip with the same sizing as styleButton.
func styleChipButton(btn *material.ButtonStyle, active bool) {
	btn.TextSize = buttonFontSize
	btn.Inset = buttonInset
	btn.CornerRadius = unit.Dp(4)
	if active {
		btn.Background = textPrimary
		btn.Color = bgPage
	} else {
		btn.Background = bgPanel
		btn.Color = textSecondary
	}
}

// uniformButton lays out a material button with a forced minimum height, so all
// buttons are visually the same height regardless of font metrics (CJK vs Latin
// glyphs have different ascent/descent). The caller passes a styled ButtonStyle
// whose Layout method is invoked.
func uniformButton(gtx layout.Context, btn *material.ButtonStyle) layout.Dimensions {
	// Force a minimum height by constraining the layout context.
	minH := gtx.Dp(buttonMinHeight)
	gtx.Constraints.Min.Y = minH
	return btn.Layout(gtx)
}
