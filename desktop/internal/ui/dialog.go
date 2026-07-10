package ui

import (
	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/graydovee/todo-manager/desktop/internal/i18n"
	"github.com/graydovee/todo-manager/desktop/internal/platform"
)

// ChoiceOption is one button in a choice dialog.
type ChoiceOption struct {
	Label string
	Value string
}

// ShowChoiceDialog opens a titled choice dialog (a secondary, owned window) with
// the given options and returns a channel that receives exactly one value: the
// chosen option's Value, or "" if the user closed the window without choosing.
// The channel is closed right after the value is delivered.
//
// The dialog is centred on the monitor's work area and made an owned child of
// the main window (via platform.SetupChildWindow) so Win32 guarantees it stays
// above the main window even when that window is top-most.
func ShowChoiceDialog(a *App, title, hint string, options []ChoiceOption) <-chan string {
	th := a.Theme
	buttons := make([]widget.Clickable, len(options))

	spec := WindowSpec{
		Title:     title,
		Width:     unit.Dp(320),
		Height:    unit.Dp(160),
		Decorated: true,
		Owner:     a.OwnerHandle,
		OnReady: func(hwnd platform.Handle, _ *app.Window) {
			platform.SetupChildWindow(hwnd, a.OwnerHandle)
		},
		OnEvent: func(gtx layout.Context, w *app.Window) string {
			for i, opt := range options {
				for buttons[i].Clicked(gtx) {
					go w.Perform(system.ActionClose) // close asynchronously so this frame still paints
					return opt.Value
				}
			}
			return ""
		},
		Render: func(gtx layout.Context) {
			paint.Fill(gtx.Ops, bgPage)
			layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							t := material.H6(th, title)
							t.Color = textPrimary
							return t.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							h := material.Body1(th, hint)
							h.Color = textSecondary
							return h.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							children := make([]layout.FlexChild, 0, len(options)*2-1)
							for i, opt := range options {
								i, opt := i, opt
								if i > 0 {
									children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout))
								}
								children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return choiceButton(gtx, th, &buttons[i], opt.Label)
								}))
							}
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
						}),
					)
				})
			})
		},
	}

	return RunWindow(spec)
}

// ConfirmCloseDialog is the specific dialog shown when the user clicks the main
// window's close button: it offers minimize, close, and cancel. It returns a
// channel delivering "minimize", "close", or "" (cancelled/closed).
func ConfirmCloseDialog(a *App) <-chan string {
	return ShowChoiceDialog(a, i18n.T("list.closeTitle"), i18n.T("list.closeHint"), []ChoiceOption{
		{Label: i18n.T("list.minimize"), Value: "minimize"},
		{Label: i18n.T("list.close"), Value: "close"},
		{Label: i18n.T("common.cancel"), Value: ""},
	})
}

// choiceButton renders a uniformly-styled dialog button.
func choiceButton(gtx layout.Context, th *material.Theme, c *widget.Clickable, label string) layout.Dimensions {
	btn := material.Button(th, c, label)
	styleButton(&btn)
	return uniformButton(gtx, &btn)
}
