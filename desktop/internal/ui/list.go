package ui

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/graydovee/todo-manager/desktop/internal/client"
	"github.com/graydovee/todo-manager/desktop/internal/i18n"
	"github.com/graydovee/todo-manager/desktop/internal/store"
)

// Backend status string values (the backend uses snake_case status names).
const (
	statusOpen       = "open"
	statusInProgress = "in_progress"
	statusCompleted  = "completed"
	statusDuplicate  = "duplicate"
)

// ListView renders the todo list, the custom top bar (title + action buttons),
// and the list footer (item count / loading indicator). Rows are virtualised
// via widget.List.
type ListView struct {
	app *App

	topBar *topBar
	list   *widget.List
	footer *widget.Label
	root   *fyne.Container

	items []client.Todo // cached snapshot for the list callbacks
}

func newListView(app *App) *ListView {
	v := &ListView{app: app}

	v.topBar = newTopBar(app, v)
	v.footer = widget.NewLabel("")
	v.footer.TextStyle = fyne.TextStyle{Italic: true}

	v.list = widget.NewList(
		v.listLength,
		v.listItemCreate,
		v.listItemUpdate,
	)
	v.list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(v.items) {
			return
		}
		t := v.items[id]
		v.app.OpenDetail(t.ID)
		// Don't keep the row visually pressed; selection highlight is driven
		// separately by SelectedID comparison in the row renderer.
		v.list.UnselectAll()
	}

	body := container.NewBorder(nil, v.footer, nil, nil, v.list)
	v.root = body
	return v
}

// Build returns the list body (without the top bar, which is mounted separately
// by App via list.BuildTopBar).
func (v *ListView) Build() fyne.CanvasObject {
	return v.root
}

// BuildTopBar returns the custom top bar widget.
func (v *ListView) BuildTopBar() fyne.CanvasObject {
	return v.topBar.Build()
}

// Refresh re-reads the store snapshot and updates the list + footer.
func (v *ListView) Refresh() {
	items, total, loading, err := v.app.Todos.Snapshot()
	v.items = items
	v.list.Refresh()
	v.RefreshHeader()
	if loading {
		v.footer.SetText(i18n.T("common.loading"))
		return
	}
	if err != nil {
		v.footer.SetText(i18n.T("common.error") + err.Error())
		return
	}
	v.footer.SetText(i18n.T("common.items", "count", total))
}

// RefreshHeader updates only the footer/title (used by SetMessage).
func (v *ListView) RefreshHeader() {
	_, total, loading, err := v.app.Todos.Snapshot()
	if loading {
		v.footer.SetText(i18n.T("common.loading"))
		return
	}
	if err != nil {
		v.footer.SetText(i18n.T("common.error") + err.Error())
		return
	}
	v.footer.SetText(i18n.T("common.items", "count", total))
}

// listLength returns the row count for the virtualised list.
func (v *ListView) listLength() int {
	return len(v.items)
}

// listItemCreate builds a fresh row widget. We use the custom listRow widget
// (rather than a raw container) so the row owns its renderer and reliably
// re-flows the title label when SetTodo changes it.
func (v *ListView) listItemCreate() fyne.CanvasObject {
	return newListRow(v.app)
}

// listItemUpdate populates the row at idx with the snapshot item.
func (v *ListView) listItemUpdate(id widget.ListItemID, obj fyne.CanvasObject) {
	if id < 0 || id >= len(v.items) {
		return
	}
	row, ok := obj.(*listRow)
	if !ok {
		return
	}
	row.SetTodo(v.items[id])
}

// doStart starts the todo and refreshes the list.
func (v *ListView) doStart(id uint) {
	v.actionWithCascade(id, "start")
}

// doComplete completes the todo. If a 409 conflict is returned, the conflict
// is shown to the user with a cascade option.
func (v *ListView) doComplete(id uint) {
	v.actionWithCascade(id, "complete")
}

// actionWithCascade runs a start/complete action against the backend.
func (v *ListView) actionWithCascade(id uint, action string) {
	c := v.app.State.Client
	if c == nil {
		return
	}
	idStr := store.IDString(id)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var err error
		switch action {
		case "start":
			_, err = c.StartTodo(ctx, idStr)
		case "complete":
			_, err = c.CompleteTodo(ctx, idStr, false)
		}
		if err != nil {
			if conf, ok := client.IsConflict(err); ok {
				fyne.Do(func() {
					showConflictDialog(v.app, id, conf, action)
				})
				return
			}
			v.app.SetMessage(i18n.T("list.actionFailed") + err.Error())
			return
		}
		fyne.Do(func() {
			v.app.refreshList()
		})
	}()
}

// priorityLabel renders a priority code (p0..p3) for the list row.
func priorityLabel(p string) string {
	p = strings.ToLower(strings.TrimSpace(p))
	if p == "" {
		return ""
	}
	return strings.ToUpper(p)
}

// statusLabel maps a backend status string to a translated label.
func statusLabel(s string) string {
	switch s {
	case statusOpen:
		return i18n.T("todo.open")
	case statusInProgress:
		return i18n.T("todo.inProgress")
	case statusCompleted:
		return i18n.T("todo.completed")
	case statusDuplicate:
		return i18n.T("todo.duplicate")
	}
	return s
}

// showConflictDialog displays a 409 dependency conflict with a cascade option.
// Choosing cascade re-runs the action with the cascade flag set.
func showConflictDialog(app *App, id uint, conf *client.ConflictResponse, action string) {
	msg := conf.Error
	if msg == "" {
		msg = i18n.T("list.blocked")
	}
	if len(conf.PendingDependencies) > 0 {
		var names []string
		for _, d := range conf.PendingDependencies {
			names = append(names, formatDisplayCode(d.Category, d.Code))
		}
		msg += "\n" + strings.Join(names, ", ")
	}
	cascadeBtn := widget.NewButton(i18n.T("detail.cascade"), nil)
	dismissBtn := widget.NewButton(i18n.T("detail.dismiss"), nil)
	dlg := widget.NewModalPopUp(container.NewVBox(
		widget.NewLabelWithStyle(msg, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(cascadeBtn, dismissBtn),
	), app.Window.Canvas())
	cascadeBtn.OnTapped = func() {
		dlg.Hide()
		c := app.State.Client
		idStr := store.IDString(id)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			var err error
			if action == "complete" {
				_, err = c.CompleteTodo(ctx, idStr, true)
			}
			if err != nil {
				app.SetMessage(i18n.T("list.actionFailed") + err.Error())
				return
			}
			fyne.Do(func() { app.refreshList() })
		}()
	}
	dismissBtn.OnTapped = func() { dlg.Hide() }
	dlg.Show()
}

// --- listRow custom widget ------------------------------------------------

// listRow is a single virtualised list row. It is a BaseWidget so we can tint
// its background to indicate the currently-open detail.
type listRow struct {
	widget.BaseWidget
	app *App

	id        uint
	title     *widget.Label // combined "CODE  Title" (bold, ellipsis-truncated)
	priority  *widget.Label
	status    *widget.Label
	actionBtn *widget.Button

	bg      *canvas.Rectangle
	sep     *canvas.Line
	content *fyne.Container
}

// newListRow constructs a row with all child widgets laid out.
func newListRow(app *App) *listRow {
	r := &listRow{app: app}
	r.title = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	r.title.Truncation = fyne.TextTruncateEllipsis
	r.priority = widget.NewLabel("")
	r.status = widget.NewLabel("")
	r.actionBtn = widget.NewButton("", nil)
	r.actionBtn.Importance = widget.LowImportance

	r.bg = canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF})
	r.sep = canvas.NewLine(color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF})
	r.sep.StrokeWidth = 1

	// Title (combined "CODE  Title") is the border CENTER so it absorbs leftover
	// width and ellipsis-truncates only when truly too long; the priority/status
	// trailing column is the border RIGHT and stays at its MinSize.
	middle := container.NewHBox(r.priority, r.status)
	trailing := container.NewHBox(middle, r.actionBtn)
	r.content = container.NewBorder(nil, nil, nil, trailing, r.title)

	r.ExtendBaseWidget(r)
	return r
}

// CreateRenderer renders the background tint, the row content, and a hairline.
func (r *listRow) CreateRenderer() fyne.WidgetRenderer {
	return &listRowRenderer{
		row:     r,
		objects: []fyne.CanvasObject{r.bg, r.content, r.sep},
	}
}

type listRowRenderer struct {
	row     *listRow
	objects []fyne.CanvasObject
}

func (r *listRowRenderer) Layout(size fyne.Size) {
	r.row.bg.Resize(size)
	r.row.content.Resize(size.Add(fyne.NewSize(-16, -8)))
	r.row.content.Move(fyne.NewPos(8, 4))
	r.row.sep.Resize(fyne.NewSize(size.Width, 1))
	r.row.sep.Move(fyne.NewPos(0, size.Height-1))
}

func (r *listRowRenderer) MinSize() fyne.Size {
	return r.row.content.MinSize().Add(fyne.NewSize(16, 8))
}

func (r *listRowRenderer) Refresh() {
	if r.row.app.State.SelectedID == r.row.id {
		r.row.bg.FillColor = color.NRGBA{R: 0xEE, G: 0xEE, B: 0xEE, A: 0xFF}
	} else {
		r.row.bg.FillColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	}
	r.row.bg.Refresh()
	r.row.content.Refresh()
}

func (r *listRowRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *listRowRenderer) Destroy()                     {}

// SetTodo populates the row with a todo snapshot and refreshes.
func (r *listRow) SetTodo(t client.Todo) {
	r.id = t.ID

	// Combine code and title into a single bold label ("F-1  Fix login bug"),
	// matching the detail panel format. Falls back to "(untitled)" when the
	// title is empty.
	title := t.Title
	if title == "" {
		title = "(untitled)"
	}
	r.title.SetText(fmt.Sprintf("%s  %s", formatDisplayCode(t.Category, t.Code), title))
	r.priority.SetText(priorityLabel(t.Priority))
	r.status.SetText(statusLabel(t.Status))

	// Bold the status when it's "in progress" to give visual weight.
	if t.Status == statusInProgress {
		r.status.TextStyle = fyne.TextStyle{Bold: true}
	} else {
		r.status.TextStyle = fyne.TextStyle{}
	}
	r.status.Refresh()

	switch t.Status {
	case statusCompleted, statusDuplicate:
		r.actionBtn.Hidden = true
		r.actionBtn.OnTapped = nil
	case statusInProgress:
		r.actionBtn.Hidden = false
		r.actionBtn.Text = "✓"
		id := t.ID
		r.actionBtn.OnTapped = func() {
			r.app.list.doComplete(id)
		}
	default: // open
		r.actionBtn.Hidden = false
		r.actionBtn.Text = "▶"
		id := t.ID
		r.actionBtn.OnTapped = func() {
			r.app.list.doStart(id)
		}
	}
	r.actionBtn.Refresh()

	// Trigger a background refresh to update the tint.
	r.Refresh()
}

// --- top bar ---------------------------------------------------------------

// topBar is the custom window title bar with action buttons.
type topBar struct {
	app  *App
	view *ListView

	root *fyne.Container

	title      *widget.Label
	createBtn  *widget.Button
	refreshBtn *widget.Button
	pinBtn     *widget.Button
	lockBtn    *widget.Button
	manageBtn  *widget.Button
	closeBtn   *widget.Button
}

func newTopBar(app *App, view *ListView) *topBar {
	t := &topBar{app: app, view: view}
	t.title = widget.NewLabelWithStyle(i18n.T("list.title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	t.createBtn = widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
		t.app.OpenCreate()
	})
	t.createBtn.Importance = widget.LowImportance

	t.refreshBtn = widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		t.app.refreshList()
	})
	t.refreshBtn.Importance = widget.LowImportance

	t.pinBtn = widget.NewButtonWithIcon("", theme.VisibilityIcon(), func() {
		t.app.SetTopMost(!t.app.State.TopMost)
		t.Refresh()
	})
	t.pinBtn.Importance = widget.LowImportance

	t.lockBtn = widget.NewButtonWithIcon("", lockIcon(), func() {
		t.app.SetLock(!t.app.State.Locked)
		t.Refresh()
	})
	t.lockBtn.Importance = widget.LowImportance

	t.manageBtn = widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		t.app.OpenManage()
	})
	t.manageBtn.Importance = widget.LowImportance

	t.closeBtn = widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		t.app.Window.Close() // triggers close intercept
	})
	t.closeBtn.Importance = widget.LowImportance

	buttons := container.NewHBox(
		t.createBtn, t.refreshBtn, t.pinBtn, t.lockBtn, t.manageBtn, t.closeBtn,
	)
	// Wrap the title in a dragBar that fills the whole space left of the
	// buttons, so the title text and the empty title-bar area can both be
	// dragged to move the borderless window.
	titleDrag := newDragBar(t.app, t.title)
	t.root = container.NewBorder(nil, nil, nil, buttons, titleDrag)
	t.Refresh()
	return t
}

// Build returns the top bar's root container.
func (t *topBar) Build() fyne.CanvasObject {
	return t.root
}

// Refresh updates button icons based on current state.
func (t *topBar) Refresh() {
	if t.app.State.TopMost {
		t.pinBtn.Icon = theme.VisibilityIcon()
	} else {
		t.pinBtn.Icon = theme.VisibilityOffIcon()
	}
	if t.app.State.Locked {
		t.lockBtn.Icon = lockIcon()
	} else {
		t.lockBtn.Icon = unlockIcon()
	}
	t.pinBtn.Refresh()
	t.lockBtn.Refresh()
}

// lockIcon returns a simple lock SVG icon resource (Fyne has no built-in lock icon).
var lockIconRes = fyne.NewStaticResource("lock.svg", []byte(
	`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>`))

// unlockIconRes returns an unlocked padlock SVG icon.
var unlockIconRes = fyne.NewStaticResource("unlock.svg", []byte(
	`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 9.9-1"/></svg>`))

// chevronLeftRes is a left-pointing chevron used for the side-panel collapse
// button, matching the line-style of the lock icons above.
var chevronLeftRes = fyne.NewStaticResource("chevron-left.svg", []byte(
	`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg>`))

func lockIcon() fyne.Resource    { return lockIconRes }
func unlockIcon() fyne.Resource  { return unlockIconRes }
func chevronLeft() fyne.Resource { return chevronLeftRes }

// time.Now kept referenced for possible future use (refresh timestamp).
var _ = time.Now
