// Package ui renders the desktop GUI. It holds the minimalist light theme and
// all page layouts.
package ui

import (
	"image/color"

	"gioui.org/font/gofont"
	"gioui.org/text"
	"gioui.org/widget/material"

	"github.com/graydovee/todo-manager/desktop/internal/i18n"
)

// Minimalist greyscale palette. The app is intentionally monochrome: states and
// priorities are distinguished by weight, shading and borders rather than colour.
var (
	// Surfaces (light theme).
	bgPage      = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} // page background
	bgPanel     = color.NRGBA{R: 0xFA, G: 0xFA, B: 0xFA, A: 0xFF} // inset panels
	bgRowAlt    = color.NRGBA{R: 0xF7, G: 0xF7, B: 0xF7, A: 0xFF} // alternating rows
	bgRowSelect = color.NRGBA{R: 0xEF, G: 0xEF, B: 0xF2, A: 0xFF} // selected row

	// Glass: shown only when the window is locked. Semi-transparent light so the
	// desktop shows through (alpha < 255).
	bgGlass = color.NRGBA{R: 0xFA, G: 0xFA, B: 0xFC, A: 0xC7} // ~0.78 alpha

	// Lines / borders.
	border     = color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF}
	borderSoft = color.NRGBA{R: 0xEE, G: 0xEE, B: 0xEE, A: 0xFF}

	// Text. Greyscale only.
	textPrimary   = color.NRGBA{R: 0x1A, G: 0x1A, B: 0x1A, A: 0xFF}
	textSecondary = color.NRGBA{R: 0x66, G: 0x66, B: 0x66, A: 0xFF}
	textMuted     = color.NRGBA{R: 0x99, G: 0x99, B: 0x99, A: 0xFF}
	textDisabled  = color.NRGBA{R: 0xBD, G: 0xBD, B: 0xBD, A: 0xFF}

	// Priority shading: darker == more urgent. Applied as text colour for Pn.
	priorityShade = [4]color.NRGBA{
		{R: 0x1A, G: 0x1A, B: 0x1A, A: 0xFF}, // p0 darkest
		{R: 0x4D, G: 0x4D, B: 0x4D, A: 0xFF}, // p1
		{R: 0x80, G: 0x80, B: 0x80, A: 0xFF}, // p2
		{R: 0xB3, G: 0xB3, B: 0xB3, A: 0xFF}, // p3 lightest
	}
)

// NewTheme returns a compact, monochrome Material theme tuned for a small
// always-on-top desktop widget.
func NewTheme() *material.Theme {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	// Compact sizing for a small window.
	th.TextSize = 13 // sp
	return th
}

// PriorityColor maps a priority string (p0..p3) to a greyscale. Unknown values
// fall back to the lightest shade.
func PriorityColor(priority string) color.NRGBA {
	switch priority {
	case "p0":
		return priorityShade[0]
	case "p1":
		return priorityShade[1]
	case "p2":
		return priorityShade[2]
	default:
		return priorityShade[3]
	}
}

// StatusLabel maps a backend status token to a short, localised human label.
func StatusLabel(status string) string {
	switch status {
	case "open":
		return i18n.T("todo.open")
	case "in_progress":
		return i18n.T("todo.inProgress")
	case "completed":
		return i18n.T("todo.completed")
	case "duplicate":
		return i18n.T("todo.duplicate")
	default:
		return status
	}
}

// CategoryPrefix maps a category to its single-letter code badge prefix.
func CategoryPrefix(category string) string {
	switch category {
	case "bug":
		return "B"
	case "feature":
		return "F"
	case "task":
		return "T"
	default:
		return "?"
	}
}

// DisplayCode renders "B-12" style codes from a category + raw code number.
func DisplayCode(category, code string) string {
	return CategoryPrefix(category) + "-" + code
}
