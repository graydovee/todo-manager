// Package ui contains the Fyne UI for the desktop todo client.
//
// The app uses a single borderless window. The left half is the todo list; the
// right half is a side panel that can show detail / manage / create and that
// collapses to widen the list. A custom top bar carries the title and the
// create / refresh / pin / lock / manage / close buttons.
package ui

import (
	"context"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"

	"github.com/graydovee/todo-manager/desktop/internal/i18n"
	"github.com/graydovee/todo-manager/desktop/internal/platform"
	"github.com/graydovee/todo-manager/desktop/internal/store"
)

// SideMode identifies what the side panel is showing.
type SideMode int

const (
	SideNone SideMode = iota
	SideDetail
	SideManage
	SideCreate
)

// App is the central UI controller. It owns the Fyne window, the stores, and
// the child views, and exposes actions invoked from the top bar, tray, list
// rows, and detail/manage panels.
type App struct {
	App     fyne.App
	Window  fyne.Window
	State   *store.AppState
	Todos   *store.TodoStore
	HomeDir string

	// Views.
	login  *LoginView
	list   *ListView
	detail *DetailView
	manage *ManageView

	// Layout containers.
	root       *fyne.Container
	side       *container.Split // HSplit between list and side panel
	sideHost   *fyne.Container  // wraps the current side panel content
	sideChrome *sidePanel       // currently mounted side-panel chrome (or nil)

	// Side panel state.
	sideVisible bool
	sideMode    SideMode

	// Platform glue.
	platform platform.Platform
	tray     *platform.Tray

	// Internal signals.
	stopDock     chan struct{}
	stopDockOnce sync.Once

	// Dock poll state (only accessed from dockTick on the poll goroutine).
	cursorLeftSince int64
}

// New constructs the App controller and builds its UI tree. It does NOT start
// the dock poll loop or show the window; call Run() for that.
func New(fyneApp fyne.App, win fyne.Window, st *store.AppState, todos *store.TodoStore, homeDir string) *App {
	a := &App{
		App:      fyneApp,
		Window:   win,
		State:    st,
		Todos:    todos,
		HomeDir:  homeDir,
		sideMode: SideNone,
		stopDock: make(chan struct{}),
	}
	a.platform = platform.New(win)

	// Build child views. They each hold a back-reference to App for actions.
	a.login = newLoginView(a)
	a.list = newListView(a)
	a.detail = newDetailView(a)
	a.manage = newManageView(a)

	// Side panel host starts empty; content is set when a side panel opens.
	a.sideHost = container.NewStack()
	a.side = container.NewHSplit(a.list.Build(), a.sideHost)
	a.side.SetOffset(1.0) // 1.0 = right collapsed; left gets everything

	a.root = container.NewBorder(a.list.BuildTopBar(), nil, nil, nil, a.side)
	win.SetContent(a.root)

	// Apply the loaded window mode (topmost / lock) before showing.
	a.applyWindowMode()

	// Intercept close: ask minimize-vs-quit instead of quitting outright.
	win.SetCloseIntercept(a.onCloseIntercept)

	return a
}

// SetTray wires the system tray. Called from main.go after creating the tray.
func (a *App) SetTray(t *platform.Tray) {
	a.tray = t
	a.syncTrayLabels()
}

// syncTrayLabels refreshes the tray menu item labels with the current language.
func (a *App) syncTrayLabels() {
	if a.tray == nil {
		return
	}
	a.tray.SetLock(a.State.Locked, i18n.T("tray.lock"))
	a.tray.SetTopMost(a.State.TopMost, i18n.T("tray.topMost"))
	a.tray.SetQuitLabel(i18n.T("tray.quit"))
}

// Run starts the dock poll loop. Call after the window has been shown.
func (a *App) Run() {
	go a.dockLoop()
}

// Stop signals background loops to exit (called from main on quit). Safe to
// call multiple times.
func (a *App) Stop() {
	a.stopDockOnce.Do(func() {
		close(a.stopDock)
	})
}

// showPage switches the root content between login and the main layout. It is
// invoked after a successful login or a logout.
func (a *App) showPage() {
	a.State.Lock()
	page := a.State.Page
	a.State.Unlock()

	if page == store.PageLogin {
		a.Window.SetContent(a.login.Build())
		a.Window.SetTitle(i18n.T("login.title"))
		return
	}
	a.Window.SetContent(a.root)
	a.Window.SetTitle(i18n.T("list.title"))
	a.refreshList()
}

// refreshList triggers a list fetch using the current filters. Safe to call
// from any goroutine; UI refresh happens via fyne.Do.
func (a *App) refreshList() {
	c := a.State.Client
	if c == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	query := a.currentQuery()
	a.Todos.Refresh(ctx, c, query, func() {
		cancel()
		a.list.Refresh()
	})
}

// currentQuery builds the backend query map from the persisted filters.
func (a *App) currentQuery() map[string][]string {
	a.State.Lock()
	defer a.State.Unlock()
	f := a.State.Config.Filters
	q := map[string][]string{}
	add := func(key string, vals []string) {
		for _, v := range vals {
			if v != "" {
				q[key] = append(q[key], v)
			}
		}
	}
	add("status", f.Status)
	add("category", f.Category)
	add("priority", f.Priority)
	if f.Query != "" {
		q["q"] = []string{f.Query}
	}
	if f.Code != "" {
		q["code"] = []string{f.Code}
	}
	sb := f.SortBy
	if sb == "" {
		sb = "created_at"
	}
	so := f.SortOrder
	if so == "" {
		so = "desc"
	}
	q["sort_by"] = []string{sb}
	q["sort_order"] = []string{so}
	q["page"] = []string{"1"}
	q["page_size"] = []string{"100"}
	return q
}

// OpenDetail opens the detail side panel for the given todo id.
func (a *App) OpenDetail(id uint) {
	a.State.Lock()
	a.State.SelectedID = id
	a.State.Unlock()

	a.sideMode = SideDetail
	a.showSidePanel(a.detail.Build(), i18n.T("detail.title"))

	idStr := store.IDString(id)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	a.Todos.LoadDetail(ctx, a.State.Client, idStr, func() {
		cancel()
		fyne.Do(func() {
			a.detail.Refresh()
		})
	})
}

// OpenManage opens the manage side panel (filters / search / language / logout).
func (a *App) OpenManage() {
	a.sideMode = SideManage
	a.manage.Refresh()
	a.showSidePanel(a.manage.Build(), i18n.T("manage.title"))
}

// OpenCreate opens the create-todo side panel.
func (a *App) OpenCreate() {
	a.sideMode = SideCreate
	a.manage.ShowCreate()
	a.showSidePanel(a.manage.BuildCreate(), i18n.T("manage.newTodo"))
}

// ToggleSidePanel collapses or expands the side panel.
func (a *App) ToggleSidePanel() {
	if a.sideVisible {
		a.hideSidePanel()
	} else {
		// Default re-open is detail of selected (if any), else manage.
		if a.State.SelectedID != 0 {
			a.OpenDetail(a.State.SelectedID)
		} else {
			a.OpenManage()
		}
	}
}

// IsSidePanelVisible reports whether the side panel is currently expanded.
func (a *App) IsSidePanelVisible() bool { return a.sideVisible }

// hideSidePanel collapses the right side of the HSplit.
func (a *App) hideSidePanel() {
	a.sideVisible = false
	a.sideMode = SideNone
	a.side.SetOffset(1.0)
	a.refreshList()
}

// showSidePanel expands the side panel and mounts the given content. The HSplit
// offset is moved to give the side panel roughly 55% of the width when there
// is enough room. The content is wrapped in a sidePanel chrome that provides
// the mode title bar and a "<" collapse button.
func (a *App) showSidePanel(content fyne.CanvasObject, title string) {
	var onEdit func()
	var editLabel string
	if a.sideMode == SideDetail {
		editLabel = i18n.T("common.edit")
		onEdit = func() {
			if a.detail.IsEditing() {
				a.detail.saveEdit()
			} else {
				a.detail.enterEdit()
			}
			// Refresh the side panel chrome so its edit button label updates.
			if a.sideChrome != nil {
				if a.detail.IsEditing() {
					a.sideChrome.SetEditLabel(i18n.T("common.save"))
				} else {
					a.sideChrome.SetEditLabel(i18n.T("common.edit"))
				}
			}
		}
	}
	sp := newSidePanel(a, title, content, onEdit)
	sp.SetEditLabel(editLabel)
	a.sideChrome = sp
	a.sideHost.Objects = []fyne.CanvasObject{sp.root}
	a.sideHost.Refresh()
	a.sideVisible = true
	a.side.SetOffset(0.45) // 0.45 = left gets 45%, side panel gets 55%
	if a.list != nil && a.list.topBar != nil {
		a.list.topBar.Refresh()
	}
}

// SetLock toggles the lock state and updates platform + tray.
func (a *App) SetLock(locked bool) {
	a.State.Lock()
	a.State.Locked = locked
	if locked {
		a.State.TopMost = true // lock implies topmost
	}
	cfg := a.State.Config
	cfg.Window.Locked = locked
	if locked {
		cfg.Window.TopMost = true
	}
	a.State.Unlock()

	a.applyWindowMode()
	a.syncTrayLabels()
}

// SetTopMost toggles the always-on-top state and updates platform + tray.
func (a *App) SetTopMost(top bool) {
	a.State.Lock()
	a.State.TopMost = top
	a.State.Config.Window.TopMost = top
	// Unlocking topmost implicitly unlocks lock (lock requires topmost).
	if !top {
		a.State.Locked = false
		a.State.Config.Window.Locked = false
	}
	a.State.Unlock()

	a.applyWindowMode()
	a.syncTrayLabels()
}

// applyWindowMode pushes the current TopMost / Locked state to the platform
// implementation. Safe to call repeatedly.
func (a *App) applyWindowMode() {
	a.State.Lock()
	top := a.State.TopMost
	locked := a.State.Locked
	a.State.Unlock()
	a.platform.SetAlwaysOnTop(top)
	a.platform.SetLock(locked)
}

// onCloseIntercept is invoked when the user clicks the close button. We show a
// confirmation dialog offering "minimize to tray" or "close".
func (a *App) onCloseIntercept() {
	d := dialog.NewConfirm(
		i18n.T("list.closeTitle"),
		i18n.T("list.closeHint"),
		func(closeApp bool) {
			if closeApp {
				a.quit()
			} else {
				a.platform.Minimize()
			}
		},
		a.Window,
	)
	d.SetConfirmText(i18n.T("list.close"))
	d.SetDismissText(i18n.T("list.minimize"))
	d.Show()
}

// quit stops background work and exits the app. Config is persisted first.
func (a *App) quit() {
	a.Stop()
	_ = a.persistConfig()
	a.App.Quit()
}

// Quit is the public version of quit, used by the tray's Quit action.
func (a *App) Quit() { a.quit() }

// persistConfig writes the current state to disk.
func (a *App) persistConfig() error {
	// Capture window geometry into config.
	if x, y, w, h := a.platform.WindowGeometry(); w > 0 && h > 0 {
		a.State.Lock()
		a.State.Config.Window.Width = w
		a.State.Config.Window.Height = h
		_ = x
		_ = y
		cfg := a.State.Config
		a.State.Unlock()
		return writeConfig(a.HomeDir, cfg)
	}
	a.State.Lock()
	cfg := a.State.Config
	a.State.Unlock()
	return writeConfig(a.HomeDir, cfg)
}

// ShowError displays a transient error dialog. Safe from any goroutine.
func (a *App) ShowError(msg string) {
	fyne.Do(func() {
		dialog.NewInformation(i18n.T("common.error"), msg, a.Window).Show()
	})
}

// ShowMessage displays a transient info dialog from any goroutine.
func (a *App) ShowMessage(title, msg string) {
	fyne.Do(func() {
		dialog.NewInformation(title, msg, a.Window).Show()
	})
}

// SetMessage updates the state's transient status banner and the list header.
// Safe to call from any goroutine; UI work is dispatched via fyne.Do.
func (a *App) SetMessage(msg string) {
	a.State.SetMessage(msg)
	fyne.Do(func() {
		a.list.RefreshHeader()
	})
}
