// Package ui contains the Fyne UI for the desktop todo client.
//
// The app uses a single borderless window. The left column is a fixed-width
// todo list; a side panel (detail / manage / create) opens to its right by
// widening the whole window, so the list never loses width. A custom top bar
// carries the title and the create / refresh / pin / lock / manage / close
// buttons.
package ui

import (
	"context"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"

	"github.com/graydovee/todo-manager/desktop/internal/i18n"
	"github.com/graydovee/todo-manager/desktop/internal/platform"
	"github.com/graydovee/todo-manager/desktop/internal/store"
)

// Width budget for the two columns of the main layout, in theme-independent
// pixels. The todo list is a narrow, fixed column; the side panel (detail /
// manage / create) opens to its right by widening the whole window rather than
// by stealing width from the list.
const (
	listPanelWidth float32 = 320
	sidePanelWidth float32 = 400
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
	contentRow *fyne.Container   // horizontal row: list column + side panel host
	listCol    fyne.CanvasObject // fixed-width todo list column
	sideHost   *fyne.Container   // wraps the current side panel content
	sideChrome *sidePanel        // currently mounted side-panel chrome (or nil)

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

	// Build the two columns: a fixed-width todo list column on the left, and a
	// side panel host on the right that starts hidden. When a side panel opens
	// the whole window widens (see showSidePanel / hideSidePanel), so the list
	// never loses width. listSideLayout keeps the list at exactly listPanelWidth
	// and lets the side panel absorb all remaining horizontal space.
	a.sideHost = container.NewStack()
	a.sideHost.Hide()

	a.listCol = a.list.Build()
	a.contentRow = fyne.NewContainerWithLayout(newListSideLayout(listPanelWidth), a.listCol, a.sideHost)

	a.root = container.NewBorder(a.list.BuildTopBar(), nil, nil, nil, a.contentRow)
	win.SetContent(a.root)

	// NOTE: applyWindowMode() is deliberately NOT called here — the native
	// GLFW window does not exist yet (it is created during win.Show()), so any
	// HWND-based Win32 call would resolve HWND=0. It is applied from Run()
	// after the window has been shown.

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

// Run starts the dock poll loop and applies the persisted window mode
// (always-on-top / lock). Call after the window has been shown, so the native
// GLFW window exists and HWND-based Win32 calls resolve to a real handle.
func (a *App) Run() {
	// Apply window mode on the Fyne main goroutine after the window is up. We
	// defer via fyne.Do so the GLFW event loop has finished creating the window
	// (Show() queues creation onto the main thread).
	fyne.Do(func() {
		a.applyWindowMode()
	})
	go a.dockLoop()
}

// Stop signals background loops to exit (called from main on quit). Safe to
// call multiple times.
func (a *App) Stop() {
	a.stopDockOnce.Do(func() {
		close(a.stopDock)
	})
}

// ShowPage switches the root content between login and the main layout. It is
// invoked after a successful login or a logout.
func (a *App) ShowPage() {
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
	// The backend reads multi-value filters (status/category/priority) as a
	// single comma-joined query param and splits them itself (repo layer does
	// strings.Split(",") + SQL IN). Sending repeated params (?status=a&status=b)
	// would be silently truncated to the first value by Echo's QueryParam, so we
	// join the selected values into one comma-separated string instead.
	joinNonEmpty := func(vals []string) []string {
		var parts []string
		for _, v := range vals {
			if v != "" {
				parts = append(parts, v)
			}
		}
		if len(parts) == 0 {
			return nil
		}
		return []string{strings.Join(parts, ",")}
	}
	if v := joinNonEmpty(f.Status); v != nil {
		q["status"] = v
	}
	if v := joinNonEmpty(f.Category); v != nil {
		q["category"] = v
	}
	if v := joinNonEmpty(f.Priority); v != nil {
		q["priority"] = v
	}
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

	// Refresh the list so visible rows re-apply the selected-row tint.
	a.list.list.Refresh()

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

// hideSidePanel collapses the side panel by hiding it and shrinking the window
// back to the list-only width. The list column keeps its fixed width throughout.
func (a *App) hideSidePanel() {
	a.sideVisible = false
	a.sideMode = SideNone
	a.sideHost.Hide()
	a.sideHost.Objects = nil
	a.sideChrome = nil
	a.resizeWindowTo(listPanelWidth)
	a.refreshList()
}

// showSidePanel expands the side panel and mounts the given content. The whole
// window is widened to listPanelWidth + sidePanelWidth so the list column keeps
// its fixed width. The content is wrapped in a sidePanel chrome that provides
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
	a.sideHost.Show()
	a.resizeWindowTo(listPanelWidth + sidePanelWidth)
	if a.list != nil && a.list.topBar != nil {
		a.list.topBar.Refresh()
	}
}

// resizeWindowTo resizes the window so its content area is contentWidth wide,
// keeping the current height. The top-left corner is captured before the resize
// and restored afterwards so widening extends to the right (the window does not
// slide leftward). Safe to call on the UI goroutine.
func (a *App) resizeWindowTo(contentWidth float32) {
	// Capture current position + height before resizing.
	x, y, _, h := a.platform.WindowGeometry()
	height := float32(h)
	if height <= 0 {
		height = a.Window.Canvas().Size().Height
	}
	a.Window.Resize(fyne.NewSize(contentWidth, height))
	// Best-effort reposition (no-op on the stub platform; honoured on Windows).
	if x != 0 || y != 0 {
		a.platform.MoveWindow(x, y)
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
