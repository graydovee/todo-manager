package ui

import (
	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/graydovee/todo-manager/desktop/internal/i18n"
	"github.com/graydovee/todo-manager/desktop/internal/platform"
)

// sideWidth is the side window width in dp.
const sideWidth = 340

// SideMode determines what content the side window shows.
type SideMode int

const (
	SideDetail SideMode = iota // todo detail view
	SideManage                 // filters / settings
	SideCreate                 // create new todo form
)

// SideWindow is a single reusable borderless window that opens to the right of
// the main list window. It can show a todo detail, the management panel, or the
// create-todo form. At most one side window exists at a time — switching modes
// reuses the live window.
type SideWindow struct {
	app *App

	// win is the live Gio window; nil when closed.
	win  *app.Window
	ctrl platform.Controller

	// mode is the current content mode.
	mode SideMode

	// closeBtn is the custom close button in the top bar.
	closeBtn widget.Clickable

	// drag enables custom window dragging (avoids Win11 Snap).
	drag      gesture.Drag
	dragStart struct{ cursorX, cursorY, winX, winY int }

	// Sub-controllers reused across frames.
	detail  *DetailUI
	manage  *ManageUI
}

// NewSideWindow constructs the side window manager.
func NewSideWindow(a *App) *SideWindow {
	sw := &SideWindow{app: a}
	sw.detail = NewDetailUI(a)
	sw.detail.hideHeader = true
	sw.detail.onBack = sw.Close

	sw.manage = NewManageUI(a)
	sw.manage.hideHeader = true
	sw.manage.onBack = sw.Close
	return sw
}

// IsOpen reports whether the side window is currently visible.
func (sw *SideWindow) IsOpen() bool {
	return sw.win != nil
}

// IsDetailMode reports whether the side window is open AND showing a todo
// detail (used for row highlighting in the main list).
func (sw *SideWindow) IsDetailMode() bool {
	return sw.win != nil && sw.mode == SideDetail
}

// OpenDetail opens (or replaces) the side window in detail mode for the given
// todo ID.
func (sw *SideWindow) OpenDetail(todoID uint) {
	sw.app.State.Lock()
	sw.app.State.SelectedID = todoID
	sw.app.State.Unlock()

	sw.detail.editing = false
	sw.detail.pendingConflict = nil
	sw.detail.resetEditors()
	sw.app.Todos.ResetDetail()
	sw.detail.Load()

	sw.mode = SideDetail
	sw.ensureOpen()
}

// OpenManage opens (or switches) the side window to the management panel.
func (sw *SideWindow) OpenManage() {
	sw.mode = SideManage
	sw.ensureOpen()
}

// OpenCreate opens (or switches) the side window to the create-todo form.
func (sw *SideWindow) OpenCreate() {
	sw.manage.creating = true
	sw.mode = SideCreate
	sw.ensureOpen()
}

// Close shuts the side window (async).
func (sw *SideWindow) Close() {
	if sw.win != nil {
		go sw.win.Perform(system.ActionClose)
	}
}

// ensureOpen starts the window goroutine if it isn't already running, then
// invalidates to refresh content. When the window is already open, it
// invalidates the side window directly so mode switches are instant (calling
// the main window's Invalidate would not wake this window's event loop).
func (sw *SideWindow) ensureOpen() {
	if sw.win == nil {
		// Not open yet — refresh the main window (e.g. row highlight) then start.
		if sw.app.Invalidate != nil {
			sw.app.Invalidate()
		}
		go sw.run()
		return
	}
	// Already open — wake THIS window so it repaints with the new mode.
	sw.win.Invalidate()
}

// run creates and drives the side window's event loop until destruction.
func (sw *SideWindow) run() {
	w := &app.Window{}
	w.Option(app.Size(unit.Dp(sideWidth), unit.Dp(560)))
	w.Option(app.Decorated(false))
	w.Option(app.Title(sw.title()))
	sw.win = w

	var ops op.Ops
	for {
		e := w.Event()
		switch e := e.(type) {
		case app.ViewEvent:
			h := platform.ExtractHandle(e)
			if h != 0 && sw.ctrl == nil {
				sw.ctrl = platform.NewController(platform.Handle(h))
				platform.SetDialogOwner(platform.Handle(h), sw.app.OwnerHandle)
				sw.positionRightOfMain()
				sw.ctrl.HideFromTaskbar()
			}
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			sw.handleFrame(gtx, w)

			paint.Fill(gtx.Ops, bgPage)
			layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(sw.topBar),
				layout.Rigid(separator),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return sw.renderContent(gtx, w)
				}),
			)
			e.Frame(gtx.Ops)

		case app.DestroyEvent:
			sw.win = nil
			sw.ctrl = nil
			sw.app.State.Lock()
			sw.app.State.SelectedID = 0
			sw.app.State.Unlock()
			if sw.app.Invalidate != nil {
				sw.app.Invalidate()
			}
			return
		}
	}
}

// title returns the window title for the current mode.
func (sw *SideWindow) title() string {
	switch sw.mode {
	case SideManage:
		return i18n.T("manage.title")
	case SideCreate:
		return i18n.T("manage.new")
	default:
		return i18n.T("detail.title")
	}
}

// renderContent renders the body for the current mode.
func (sw *SideWindow) renderContent(gtx layout.Context, w *app.Window) layout.Dimensions {
	switch sw.mode {
	case SideManage:
		sw.manage.creating = false
		return sw.manage.Layout(gtx, w)
	case SideCreate:
		sw.manage.creating = true
		return sw.manage.Layout(gtx, w)
	default:
		return sw.detail.Layout(gtx, w)
	}
}

// positionRightOfMain places the side window immediately to the right of the
// main window, matching its height.
func (sw *SideWindow) positionRightOfMain() {
	mainCtrl := sw.app.Platform
	if mainCtrl == nil || sw.ctrl == nil {
		return
	}
	mx, my, mw, mh := mainCtrl.WindowGeometry()
	if mw == 0 {
		return
	}
	_, _, dw, _ := sw.ctrl.WindowGeometry()
	if dw == 0 {
		dw = sideWidth
	}
	sw.ctrl.MoveWindow(mx+mw, my, dw, mh)
}

// handleFrame processes the close button, top-bar action buttons, and custom
// window drag.
func (sw *SideWindow) handleFrame(gtx layout.Context, w *app.Window) {
	for sw.closeBtn.Clicked(gtx) {
		go w.Perform(system.ActionClose)
	}
	sw.handleDrag(gtx)
}

// handleDrag implements custom window dragging over the top bar.
func (sw *SideWindow) handleDrag(gtx layout.Context) {
	if sw.ctrl == nil {
		return
	}
	sw.drag.Add(gtx.Ops)
	for {
		ev, ok := sw.drag.Update(gtx.Metric, gtx.Source, gesture.Both)
		if !ok {
			break
		}
		switch {
		case ev.Kind == pointer.Press && ev.Buttons == pointer.ButtonPrimary:
			cx, cy := sw.ctrl.CursorPos()
			wx, wy, _, _ := sw.ctrl.WindowGeometry()
			sw.dragStart.cursorX = cx
			sw.dragStart.cursorY = cy
			sw.dragStart.winX = wx
			sw.dragStart.winY = wy
		case ev.Kind == pointer.Drag:
			cx, cy := sw.ctrl.CursorPos()
			dx := cx - sw.dragStart.cursorX
			dy := cy - sw.dragStart.cursorY
			sw.ctrl.MoveWindow(sw.dragStart.winX+dx, sw.dragStart.winY+dy, 0, 0)
		}
	}
}

// topBar renders the custom top bar: draggable title (left), mode-specific
// action buttons, and a close button (right).
func (sw *SideWindow) topBar(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(12), Right: unit.Dp(8)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				// Draggable title.
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					t := material.Body1(sw.app.Theme, sw.title())
					t.TextSize = unit.Sp(16)
					t.Font.Weight = font.SemiBold
					t.Color = textPrimary
					dims := t.Layout(gtx)
					sw.handleDrag(gtx)
					return dims
				}),
				// Mode-specific action buttons.
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return sw.actionButtons(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				// Close button.
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return iconButton(gtx, sw.app.Theme, &sw.closeBtn, IconClose, false)
				}),
			)
		},
	)
}

// actionButtons renders the context-appropriate buttons for the current mode.
// In detail edit mode: Save + Cancel. Otherwise nothing (the manage/create
// forms have their own buttons inside the body).
func (sw *SideWindow) actionButtons(gtx layout.Context) layout.Dimensions {
	if sw.mode == SideDetail && sw.detail.editing {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return smallButton(gtx, sw.app.Theme, &sw.detail.saveBtn, i18n.T("common.save"))
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return smallButton(gtx, sw.app.Theme, &sw.detail.cancelBtn, i18n.T("common.cancel"))
			}),
		)
	}
	if sw.mode == SideDetail && !sw.detail.editing {
		return smallButton(gtx, sw.app.Theme, &sw.detail.editBtn, i18n.T("common.edit"))
	}
	return layout.Dimensions{}
}
