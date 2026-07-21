// Package theme implements a minimalist black/white/greyscale Fyne theme for the
// desktop todo client. It deliberately avoids colour: priorities and statuses are
// distinguished by weight and shading, matching the original Gio aesthetic.
package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Palette — minimalist greyscale.
var (
	colBackground   = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} // white
	colForeground   = color.NRGBA{R: 0x1A, G: 0x1A, B: 0x1A, A: 0xFF} // near-black text
	colPanel        = color.NRGBA{R: 0xFA, G: 0xFA, B: 0xFA, A: 0xFF} // side panel / cards
	colButtonBg     = color.NRGBA{R: 0x1A, G: 0x1A, B: 0x1A, A: 0xFF} // primary (high-importance) button
	colButtonTxt    = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} // primary button text
	colButtonLight  = color.NRGBA{R: 0xEF, G: 0xEF, B: 0xEF, A: 0xFF} // default (medium-importance) button bg
	colDisabled     = color.NRGBA{R: 0xBD, G: 0xBD, B: 0xBD, A: 0xFF} // grey
	colBorder       = color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF} // hairline border
	colSeparator    = color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF}
	colHover        = color.NRGBA{R: 0xF0, G: 0xF0, B: 0xF0, A: 0xFF}
	colSelected     = color.NRGBA{R: 0xEE, G: 0xEE, B: 0xEE, A: 0xFF}
	colPlaceholder  = color.NRGBA{R: 0x9E, G: 0x9E, B: 0x9E, A: 0xFF} // hint text
	colInputBg      = color.NRGBA{R: 0xFA, G: 0xFA, B: 0xFA, A: 0xFF}
	colShadow       = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x14} // very faint
	colError        = color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF} // kept greyscale
	colSuccess      = color.NRGBA{R: 0x55, G: 0x55, B: 0x55, A: 0xFF}
	colFocus        = color.NRGBA{R: 0x1A, G: 0x1A, B: 0x1A, A: 0x33} // subtle focus ring
	colHeader       = color.NRGBA{R: 0xF5, G: 0xF5, B: 0xF5, A: 0xFF} // top bar background
	colHeaderBorder = color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF}
)

// Minimal is the application theme singleton.
var Minimal fyne.Theme = (*minimalTheme)(nil)

type minimalTheme struct{}

// Color maps a fyne theme color name to a palette colour. Unknown names fall
// back to the default theme so the app never panics.
func (t *minimalTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colBackground
	case theme.ColorNameForeground:
		return colForeground
	case theme.ColorNameButton:
		// Default (medium-importance) buttons: light background so the dark
		// foreground text stays readable. High-importance buttons fill with
		// ColorNamePrimary below.
		return colButtonLight
	case theme.ColorNameDisabledButton:
		return colDisabled
	case theme.ColorNameInputBackground:
		return colInputBg
	case theme.ColorNameDisabled:
		return colDisabled
	case theme.ColorNamePlaceHolder:
		return colPlaceholder
	case theme.ColorNamePrimary:
		// High-importance buttons fill with this and render text in
		// ColorNameBackground (white), so it must be the dark button colour.
		return colButtonBg
	case theme.ColorNameHover:
		return colHover
	case theme.ColorNameSelection:
		return colSelected
	case theme.ColorNameSeparator:
		return colSeparator
	case theme.ColorNameInputBorder:
		return colBorder
	case theme.ColorNameShadow:
		return colShadow
	case theme.ColorNameError:
		return colError
	case theme.ColorNameSuccess:
		return colSuccess
	case theme.ColorNameHeaderBackground:
		return colHeader
	case theme.ColorNameFocus:
		return colFocus
	case theme.ColorNameMenuBackground:
		return colBackground
	case theme.ColorNameScrollBar:
		return colDisabled
	}
	return theme.DefaultTheme().Color(name, theme.VariantLight)
}

// Size returns a tuned size for UI elements. The list row height and padding
// are tightened to match the dense original look.
func (t *minimalTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 5
	case theme.SizeNameInnerPadding:
		return 6
	case theme.SizeNameText:
		return 13
	case theme.SizeNameHeadingText:
		return 15
	case theme.SizeNameSubHeadingText:
		return 14
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameScrollBar:
		return 8
	case theme.SizeNameScrollBarSmall:
		return 4
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameInlineIcon:
		return 16
	case theme.SizeNameLineSpacing:
		return 4
	}
	return theme.DefaultTheme().Size(name)
}

// Font returns the font resource for a text style. We delegate to the default
// theme (which bundles a full gofont set) rather than embedding our own.
func (t *minimalTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

// Icon returns the icon resource for a name. We reuse the default theme's icons
// (they are already monochrome line icons that fit the greyscale aesthetic).
func (t *minimalTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

// Convenience helpers exposed for widgets that need raw palette colours.
func Foreground() color.Color       { return colForeground }
func Background() color.Color       { return colBackground }
func PanelBackground() color.Color  { return colPanel }
func Border() color.Color           { return colBorder }
func ButtonText() color.Color       { return colButtonTxt }
func ButtonBackground() color.Color { return colButtonBg }
func Disabled() color.Color         { return colDisabled }
func Selected() color.Color         { return colSelected }
func HeaderBackground() color.Color { return colHeader }
func Placeholder() color.Color      { return colPlaceholder }
