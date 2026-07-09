package ui

import (
	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/graydovee/todo-manager/desktop/internal/platform"
)

// choiceOption is one button in a choice dialog.
type choiceOption struct {
	Label string
	Value string
}

// showChoiceDialog opens a separate, centered window displaying a title, a
// hint, and a row of choice buttons. It blocks the calling goroutine until the
// user picks an option or closes the window, then returns the chosen value (or
// "" if closed without choosing).
//
// The dialog is made an owned window of the main todo window (ownerHandle).
// Win32 guarantees an owned window always renders above its owner regardless of
// either window's TOPMOST state — this is what keeps the dialog reachable when
// the main window is itself top-most. The dialog does NOT need TOPMOST; the
// owner relationship enforces the z-order. ActivateWindow grants it focus.
func showChoiceDialog(th *material.Theme, ownerHandle platform.Handle, title, hint string, options []choiceOption) string {
	result := make(chan string, 1)

	dlg := &app.Window{}
	dlg.Option(app.Size(unit.Dp(320), unit.Dp(160)))
	dlg.Option(app.Title(title))

	buttons := make([]widget.Clickable, len(options))
	var dlgHwnd platform.Handle
	gotHwnd := false
	setupDone := false // owner + centre + activate applied once
	closing := false
	chosen := ""

	var ops op.Ops
	for {
		e := dlg.Event()
		switch e := e.(type) {
		case app.ViewEvent:
			// Capture the native handle; needed to set owner + centre.
			h := platform.ExtractHandle(e)
			if h != 0 && !gotHwnd {
				gotHwnd = true
				dlgHwnd = platform.Handle(h)
			}
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)

			// On the first frame: establish the owner relationship so the
			// dialog stays above the main window, centre it on the work area,
			// and bring it to the foreground for keyboard focus. Doing this on
			// FrameEvent (not ViewEvent) avoids synchronous-message deadlocks.
			if !setupDone && dlgHwnd != 0 {
				setupDone = true
				if ownerHandle != 0 {
					platform.SetDialogOwner(dlgHwnd, ownerHandle)
				}
				platform.CenterWindow(dlgHwnd)
				platform.ActivateWindow(dlgHwnd)
			}

			// Process button clicks. Only honoured before a choice has been
			// made; once chosen we are just draining events until the close
			// lands.
			if !closing {
				for i, opt := range options {
					for buttons[i].Clicked(gtx) {
						chosen = opt.Value
						closing = true
						dlg.Perform(system.ActionClose)
					}
				}
			}

			// Render the dialog body even while closing so the window never
			// appears frozen.
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
									return choiceBtn(gtx, th, &buttons[i], opt.Label)
								}))
							}
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
						}),
					)
				})
			})
			e.Frame(gtx.Ops)

		case app.DestroyEvent:
			select {
			case r := <-result:
				return r
			default:
				return chosen
			}
		}
	}
}

// choiceBtn renders a uniformly-styled dialog button.
func choiceBtn(gtx layout.Context, th *material.Theme, c *widget.Clickable, label string) layout.Dimensions {
	btn := material.Button(th, c, label)
	btn.TextSize = unit.Sp(13)
	btn.Inset = layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(12), Right: unit.Dp(12)}
	btn.Background = textPrimary
	btn.Color = bgPage
	return btn.Layout(gtx)
}
