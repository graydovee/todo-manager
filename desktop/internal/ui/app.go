package ui

import (
	"strings"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/graydovee/todo-manager/desktop/internal/client"
	"github.com/graydovee/todo-manager/desktop/internal/platform"
	"github.com/graydovee/todo-manager/desktop/internal/store"
)

// App owns the per-frame UI state (input fields, scroll positions) and renders
// the current page based on store.AppState. It is rebuilt every frame by Layout.
type App struct {
	Theme *material.Theme
	State *store.AppState
	Todos *store.TodoStore

	// Platform window controller; nil until the native handle arrives.
	Platform platform.Controller
	// Invalidate triggers a repaint (passed in from main).
	Invalidate func()

	// Page sub-controllers, created once and reused across frames.
	Login  *LoginUI
	List   *ListUI
	Detail *DetailUI
	Manage *ManageUI
}

// NewApp constructs the UI controller with all sub-pages wired to the stores.
func NewApp(th *material.Theme, state *store.AppState, todos *store.TodoStore) *App {
	a := &App{Theme: th, State: state, Todos: todos}
	a.Login = NewLoginUI(a)
	a.List = NewListUI(a)
	a.Detail = NewDetailUI(a)
	a.Manage = NewManageUI(a)
	return a
}

// Layout is called on every FrameEvent. It fills the background and routes to
// the active page.
func (a *App) Layout(gtx layout.Context, w *app.Window) layout.Dimensions {
	// Background: opaque light, or translucent glass when locked.
	if a.isLocked() {
		paint.Fill(gtx.Ops, bgGlass)
	} else {
		paint.Fill(gtx.Ops, bgPage)
	}

	page := a.currentPage()
	var dims layout.Dimensions
	switch page {
	case store.PageLogin:
		dims = a.Login.Layout(gtx, w)
	case store.PageList:
		dims = a.List.Layout(gtx, w)
	case store.PageDetail:
		dims = a.Detail.Layout(gtx, w)
	case store.PageManage:
		dims = a.Manage.Layout(gtx, w)
	default:
		dims = layout.Dimensions{Size: gtx.Constraints.Max}
	}

	// Transient status banner across the bottom.
	if msg := a.message(); msg != "" {
		showBanner(gtx, a.Theme, msg)
	}
	return dims
}

func (a *App) currentPage() store.Page {
	a.State.Lock()
	defer a.State.Unlock()
	return a.State.Page
}

func (a *App) isLocked() bool {
	a.State.Lock()
	defer a.State.Unlock()
	return a.State.Locked
}

// IsLocked reports the lock state (safe for any goroutine).
func (a *App) IsLocked() bool { return a.isLocked() }

func (a *App) isTopMost() bool {
	a.State.Lock()
	defer a.State.Unlock()
	return a.State.TopMost
}

// IsTopMost reports the top-most state (safe for any goroutine).
func (a *App) IsTopMost() bool { return a.isTopMost() }

func (a *App) message() string {
	a.State.Lock()
	defer a.State.Unlock()
	return a.State.Message
}

// SetLock updates state and applies the platform window mode, then invalidates.
func (a *App) SetLock(locked bool) {
	a.State.Lock()
	a.State.Locked = locked
	if locked {
		a.State.TopMost = true // locking implies top-most
	}
	cfg := a.State.Config
	cfg.Window.Locked = locked
	if locked {
		cfg.Window.TopMost = true
	}
	a.State.Unlock()
	a.applyWindowMode()
}

// SetTopMost toggles the always-on-top state. It is a no-op to disable top-most
// while locked (lock implies top-most).
func (a *App) SetTopMost(topmost bool) {
	a.State.Lock()
	a.State.TopMost = topmost
	if !topmost && a.State.Locked {
		a.State.TopMost = true
	}
	a.State.Config.Window.TopMost = a.State.TopMost
	a.State.Unlock()
	a.applyWindowMode()
}

// applyWindowMode pushes the current lock/top-most state to the platform
// controller and requests a repaint.
func (a *App) applyWindowMode() {
	if a.Platform == nil {
		return
	}
	topmost := a.isTopMost()
	locked := a.isLocked()
	if locked {
		a.Platform.SetLock(true)
	} else {
		a.Platform.SetLock(false)
		a.Platform.SetTopMost(topmost)
	}
	if a.Invalidate != nil {
		a.Invalidate()
	}
}

// showBanner draws a small bottom-of-window status line.
func showBanner(gtx layout.Context, th *material.Theme, msg string) {
	layout.Stack{Alignment: layout.S}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = 0
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, msg)
				lbl.Color = textSecondary
				return lbl.Layout(gtx)
			})
		}),
	)
}

// navigator is a small helper passed to pages for navigation.
type navigator struct {
	app *App
}

func (n navigator) goTo(p store.Page) {
	n.app.State.Lock()
	n.app.State.Page = p
	n.app.State.Unlock()
	if n.app.Invalidate != nil {
		n.app.Invalidate()
	}
}

func (a *App) nav() navigator { return navigator{app: a} }

// Client returns the active API client, or nil when not logged in.
func (a *App) Client() *client.Client {
	a.State.Lock()
	defer a.State.Unlock()
	return a.State.Client
}

// buildListQuery converts the persisted filters into backend query params.
func (a *App) buildListQuery() map[string][]string {
	a.State.Lock()
	defer a.State.Unlock()
	f := a.State.Config.Filters
	q := map[string][]string{}
	if strings.TrimSpace(f.Query) != "" {
		q["q"] = []string{strings.TrimSpace(f.Query)}
	}
	if strings.TrimSpace(f.Code) != "" {
		q["code"] = []string{strings.TrimSpace(f.Code)}
	}
	if len(f.Status) > 0 {
		q["status"] = []string{strings.Join(f.Status, ",")}
	}
	if len(f.Category) > 0 {
		q["category"] = []string{strings.Join(f.Category, ",")}
	}
	if len(f.Priority) > 0 {
		q["priority"] = []string{strings.Join(f.Priority, ",")}
	}
	if f.SortBy != "" {
		q["sort_by"] = []string{f.SortBy}
	}
	if f.SortOrder != "" {
		q["sort_order"] = []string{f.SortOrder}
	}
	q["page"] = []string{"1"}
	q["page_size"] = []string{"100"}
	return q
}
