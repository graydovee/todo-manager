package ui

import (
	"runtime/debug"
	"sync"

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
//
// Concurrency model: the side window's event loop runs on its own goroutine
// (launched by ensureOpen). All of its mutable UI state (mode, detail.*, manage.*)
// is owned exclusively by that goroutine. The main-window goroutine (and network
// goroutines) never touch that state directly; instead they call Post to enqueue
// a function that the side goroutine drains at the top of each event-loop
// iteration. The lifecycle fields (win, ctrl, opening, mode) are guarded by mu so
// the main window can read IsOpen/IsDetailMode safely on every frame.
type SideWindow struct {
	app *App

	// theme is this window's own material theme. It owns a text.Shaper that is
	// NOT concurrency-safe, so the side window must never share a theme with the
	// main window (each runs on a separate goroutine).
	theme *material.Theme

	// mu guards the lifecycle fields below. They are read from the main-window
	// goroutine (IsOpen/IsDetailMode on every frame) and written from the side
	// goroutine (run/teardown).
	mu      sync.Mutex
	win     *app.Window // the live Gio window; nil when closed
	ctrl    platform.Controller
	opening bool // true between ensureOpen launching run() and run() exiting
	mode    SideMode

	// cmds marshals work onto the side goroutine. OpenDetail/OpenManage/OpenCreate
	// and network callbacks post closures here; run drains it before each frame.
	cmds chan func()

	// closeBtn is the custom close button in the top bar.
	closeBtn widget.Clickable

	// drag enables custom window dragging (avoids Win11 Snap).
	drag      gesture.Drag
	dragStart struct{ cursorX, cursorY, winX, winY int }

	// Sub-controllers, rendered only by this window's goroutine.
	detail *DetailUI
	manage *ManageUI
}

// NewSideWindow constructs the side window manager.
func NewSideWindow(a *App) *SideWindow {
	sw := &SideWindow{
		app:   a,
		theme: NewTheme(),
		cmds:  make(chan func(), 64),
	}
	sw.detail = NewDetailUI(a)
	sw.detail.hideHeader = true
	sw.detail.onBack = sw.Close

	sw.manage = NewManageUI(a)
	sw.manage.hideHeader = true
	sw.manage.onBack = sw.Close
	return sw
}

// IsOpen reports whether the side window is currently visible. Safe to call from
// any goroutine.
func (sw *SideWindow) IsOpen() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.win != nil
}

// IsDetailMode reports whether the side window is open AND showing a todo detail
// (used for row highlighting in the main list). Safe to call from any goroutine.
func (sw *SideWindow) IsDetailMode() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.win != nil && sw.mode == SideDetail
}

// Post schedules f to run on the side window's event-loop goroutine and wakes
// the window so the queued work is processed promptly. Use this from other
// goroutines (main window clicks, network callbacks) instead of mutating the
// side window's UI state directly. If the window is closed the callback is still
// queued and runs when the window next opens (or is dropped if the channel
// fills). Safe to call from any goroutine.
func (sw *SideWindow) Post(f func()) {
	select {
	case sw.cmds <- f:
	default:
		// Channel full: drop to avoid blocking the caller.
	}
	sw.wake()
}

// wake requests a repaint of the side window (no-op if closed).
// app.Window.Invalidate is documented as safe for concurrent use.
func (sw *SideWindow) wake() {
	sw.mu.Lock()
	w := sw.win
	sw.mu.Unlock()
	if w != nil {
		w.Invalidate()
	}
}

// OpenDetail opens (or replaces) the side window in detail mode for the given
// todo ID. Called from the main-window goroutine.
func (sw *SideWindow) OpenDetail(todoID uint) {
	sw.app.State.Lock()
	sw.app.State.SelectedID = todoID
	sw.app.State.Unlock()

	sw.Post(func() {
		sw.detail.editing = false
		sw.detail.pendingConflict = nil
		sw.detail.resetEditors()
		sw.app.Todos.ResetDetail()
		sw.detail.Load()
		sw.mu.Lock()
		sw.mode = SideDetail
		sw.mu.Unlock()
	})
	sw.ensureOpen()
}

// OpenManage opens (or switches) the side window to the management panel.
// Called from the main-window goroutine.
func (sw *SideWindow) OpenManage() {
	sw.Post(func() {
		sw.mu.Lock()
		sw.mode = SideManage
		sw.mu.Unlock()
	})
	sw.ensureOpen()
}

// OpenCreate opens (or switches) the side window to the create-todo form.
// Called from the main-window goroutine.
func (sw *SideWindow) OpenCreate() {
	sw.Post(func() {
		sw.manage.creating = true
		sw.mu.Lock()
		sw.mode = SideCreate
		sw.mu.Unlock()
	})
	sw.ensureOpen()
}

// Close shuts the side window (async).
func (sw *SideWindow) Close() {
	sw.mu.Lock()
	w := sw.win
	sw.mu.Unlock()
	if w != nil {
		go w.Perform(system.ActionClose)
	}
}

// ensureOpen starts the window goroutine if it isn't already running, then
// invalidates to refresh content. When the window is already open, it invalidates
// the side window directly so mode switches are instant.
func (sw *SideWindow) ensureOpen() {
	sw.mu.Lock()
	if sw.win != nil || sw.opening {
		w := sw.win
		sw.mu.Unlock()
		// Already open (or opening) — wake THIS window so it repaints.
		if w != nil {
			w.Invalidate()
		}
		return
	}
	sw.opening = true
	sw.mu.Unlock()

	// Refresh the main window (e.g. row highlight) for the new selection.
	if sw.app.Invalidate != nil {
		sw.app.Invalidate()
	}
	go sw.run()
}

// run creates and drives the side window's event loop until destruction. It runs
// on its own goroutine (launched by ensureOpen). Because this is a separate
// goroutine, a panic here is NOT caught by the main-goroutine recover in main.go;
// the deferred recover below records it and tears down shared state.
func (sw *SideWindow) run() {
	defer func() {
		if r := recover(); r != nil {
			Logf("PANIC in side window goroutine: %v\n%s", r, debug.Stack())
			sw.teardown()
		}
	}()

	w := &app.Window{}
	w.Option(app.Size(unit.Dp(sideWidth), unit.Dp(560)))
	w.Option(app.Decorated(false))
	w.Option(app.Title(sw.title()))

	sw.mu.Lock()
	sw.win = w
	sw.mu.Unlock()

	var ops op.Ops
	for {
		// Run any work queued by other goroutines (mode switches, detail loads,
		// network callbacks) before processing the next window event.
		sw.drainCmds()

		e := w.Event()
		switch e := e.(type) {
		case app.ViewEvent:
			h := platform.ExtractHandle(e)
			if h != 0 {
				sw.mu.Lock()
				ctrl := sw.ctrl
				sw.mu.Unlock()
				if ctrl == nil {
					sw.mu.Lock()
					sw.ctrl = platform.NewController(platform.Handle(h))
					ctrl = sw.ctrl
					sw.mu.Unlock()
					platform.SetDialogOwner(platform.Handle(h), sw.app.OwnerHandle)
					sw.positionRightOfMain(ctrl)
					ctrl.HideFromTaskbar()
				}
			}
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			sw.handleFrame(gtx, w)

			paint.Fill(gtx.Ops, bgPage)
			layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return sw.topBar(gtx) }),
				layout.Rigid(separator),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return sw.renderContent(gtx, w)
				}),
			)
			e.Frame(gtx.Ops)

		case app.DestroyEvent:
			sw.teardown()
			return
		}
	}
}

// drainCmds runs all queued callbacks on the side goroutine.
func (sw *SideWindow) drainCmds() {
	for {
		select {
		case f := <-sw.cmds:
			f()
		default:
			return
		}
	}
}

// teardown clears the shared side-window state so the main window no longer
// references the (possibly dead) window/controller. Shared by the normal
// DestroyEvent path and the panic-recovery path. Runs on the side goroutine.
func (sw *SideWindow) teardown() {
	sw.mu.Lock()
	sw.win = nil
	sw.ctrl = nil
	sw.opening = false
	sw.mu.Unlock()
	sw.app.State.Lock()
	sw.app.State.SelectedID = 0
	sw.app.State.Unlock()
	if sw.app.Invalidate != nil {
		sw.app.Invalidate()
	}
}

// title returns the window title for the current mode.
func (sw *SideWindow) title() string {
	sw.mu.Lock()
	mode := sw.mode
	sw.mu.Unlock()
	switch mode {
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
	sw.mu.Lock()
	mode := sw.mode
	sw.mu.Unlock()
	switch mode {
	case SideManage:
		sw.manage.creating = false
		return sw.manage.Layout(gtx, w, sw.theme)
	case SideCreate:
		sw.manage.creating = true
		return sw.manage.Layout(gtx, w, sw.theme)
	default:
		return sw.detail.Layout(gtx, w, sw.theme)
	}
}

// positionRightOfMain places the side window immediately to the right of the main
// window, matching its height.
func (sw *SideWindow) positionRightOfMain(ctrl platform.Controller) {
	mainCtrl := sw.app.Platform
	if mainCtrl == nil || ctrl == nil {
		return
	}
	mx, my, mw, mh := mainCtrl.WindowGeometry()
	if mw == 0 {
		return
	}
	_, _, dw, _ := ctrl.WindowGeometry()
	if dw == 0 {
		dw = sideWidth
	}
	ctrl.MoveWindow(mx+mw, my, dw, mh)
}

// handleFrame processes the close button and custom window drag.
func (sw *SideWindow) handleFrame(gtx layout.Context, w *app.Window) {
	for sw.closeBtn.Clicked(gtx) {
		go w.Perform(system.ActionClose)
	}
	sw.handleDrag(gtx)
}

// handleDrag implements custom window dragging over the top bar.
func (sw *SideWindow) handleDrag(gtx layout.Context) {
	sw.mu.Lock()
	ctrl := sw.ctrl
	sw.mu.Unlock()
	if ctrl == nil {
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
			cx, cy := ctrl.CursorPos()
			wx, wy, _, _ := ctrl.WindowGeometry()
			sw.dragStart.cursorX = cx
			sw.dragStart.cursorY = cy
			sw.dragStart.winX = wx
			sw.dragStart.winY = wy
		case ev.Kind == pointer.Drag:
			cx, cy := ctrl.CursorPos()
			dx := cx - sw.dragStart.cursorX
			dy := cy - sw.dragStart.cursorY
			ctrl.MoveWindow(sw.dragStart.winX+dx, sw.dragStart.winY+dy, 0, 0)
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
					t := material.Body1(sw.theme, sw.title())
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
					return iconButton(gtx, sw.theme, &sw.closeBtn, IconClose, false)
				}),
			)
		},
	)
}

// actionButtons renders the context-appropriate buttons for the current mode.
// In detail edit mode: Save + Cancel. Otherwise nothing (the manage/create forms
// have their own buttons inside the body).
func (sw *SideWindow) actionButtons(gtx layout.Context) layout.Dimensions {
	if sw.mode == SideDetail && sw.detail.editing {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return smallButton(gtx, sw.theme, &sw.detail.saveBtn, i18n.T("common.save"))
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return smallButton(gtx, sw.theme, &sw.detail.cancelBtn, i18n.T("common.cancel"))
			}),
		)
	}
	if sw.mode == SideDetail && !sw.detail.editing {
		return smallButton(gtx, sw.theme, &sw.detail.editBtn, i18n.T("common.edit"))
	}
	return layout.Dimensions{}
}
