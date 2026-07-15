package ui

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/graydovee/todo-manager/desktop/internal/client"
	"github.com/graydovee/todo-manager/desktop/internal/i18n"
	"github.com/graydovee/todo-manager/desktop/internal/store"
)

// ListUI is the main todo list screen.
type ListUI struct {
	app *App

	topMostBtn widget.Clickable
	lockBtn    widget.Clickable
	manageBtn  widget.Clickable
	refreshBtn widget.Clickable
	createBtn  widget.Clickable
	closeBtn   widget.Clickable

	list layout.List

	// Per-row clickables, indexed by row. Grown as needed each frame.
	rows []rowWidgets

	// Sort column header clickables.
	headerTitle widget.Clickable

	// Custom window drag (replaces system ActionMove to avoid Win11 Snap).
	drag      gesture.Drag
	dragStart struct{ cursorX, cursorY, winX, winY int }

	// firstLoad tracks whether the initial fetch has been triggered.
	firstLoad bool
}

type rowWidgets struct {
	row    widget.Clickable // whole row -> open detail
	action widget.Clickable // start / complete button
}

// Shared column widths (dp) so the header and rows stay aligned.
const (
	colPriorityW = 44
	colStatusW   = 78
	colActionW   = 52
)

func NewListUI(a *App) *ListUI {
	return &ListUI{app: a, list: layout.List{Axis: layout.Vertical}}
}

func (u *ListUI) Layout(gtx layout.Context, w *app.Window, th *material.Theme) layout.Dimensions {
	// Trigger the initial fetch the first time the list page renders.
	if !u.firstLoad {
		u.firstLoad = true
		u.RequestRefresh()
	}

	items, total, loading, err := u.app.Todos.Snapshot()

	// Grow per-row clickables to match the item count.
	if cap(u.rows) < len(items) {
		u.rows = make([]rowWidgets, len(items))
	} else {
		u.rows = u.rows[:len(items)]
	}

	// Process clicks that occurred this frame.
	u.handleTopBarClicks(gtx, w)
	u.handleRowClicks(gtx, items)

	// Build the main content, then overlay the close dialog if active.
	content := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.topBar(gtx, th) }),
		layout.Rigid(separator),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			switch {
			case err != nil:
				return centeredText(gtx, th, i18n.T("common.error")+err.Error(), textMuted)
			case loading && len(items) == 0:
				return centeredText(gtx, th, i18n.T("common.loading"), textMuted)
			case len(items) == 0:
				return centeredText(gtx, th, i18n.T("list.noTodos"), textMuted)
			default:
				return u.table(gtx, items, th)
			}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.footer(gtx, th, total, len(items), loading)
		}),
	)

	return content
}

// topBar renders the title and the pin/lock/manage icon buttons. The title text
// is a custom drag handle (gesture.Drag) that moves the native window via Win32
// SetWindowPos — this avoids registering system.ActionMove, which would make
// Gio return HTCAPTION and trigger Windows 11 Snap on edge drag.
func (u *ListUI) topBar(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(12), Right: unit.Dp(8)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				// Draggable title: the whole left area is a drag handle.
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							t := material.Body1(th, i18n.T("list.title"))
							t.TextSize = unit.Sp(16)
							t.Font.Weight = font.SemiBold
							t.Color = textPrimary
							return t.Layout(gtx)
						}),
					)
					u.handleDrag(gtx)
					return dims
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return iconButton(gtx, th, &u.createBtn, IconPlus, false) }),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return iconButton(gtx, th, &u.refreshBtn, IconRefresh, false)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return iconButton(gtx, th, &u.topMostBtn, IconPin, u.app.isTopMost())
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return iconButton(gtx, th, &u.lockBtn, IconLock, u.app.isLocked())
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return iconButton(gtx, th, &u.manageBtn, IconSettings, false)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return iconButton(gtx, th, &u.closeBtn, IconClose, false) }),
			)
		},
	)
}

// handleDrag processes custom window-drag pointer events over the title area.
// On Press it captures the cursor + window origin; on Drag it moves the window
// by the delta (constrained to the edge if docked); on Release it checks for
// edge snapping.
func (u *ListUI) handleDrag(gtx layout.Context) {
	if u.app.IsModal() {
		return
	}
	u.drag.Add(gtx.Ops)
	ctrl := u.app.Platform
	if ctrl == nil {
		return
	}
	for {
		ev, ok := u.drag.Update(gtx.Metric, gtx.Source, gesture.Both)
		if !ok {
			break
		}
		switch {
		case ev.Kind == pointer.Press && ev.Buttons == pointer.ButtonPrimary:
			cx, cy := ctrl.CursorPos()
			wx, wy, _, _ := ctrl.WindowGeometry()
			u.dragStart.cursorX = cx
			u.dragStart.cursorY = cy
			u.dragStart.winX = wx
			u.dragStart.winY = wy
			u.app.State.Dock.SetDragging(true)
		case ev.Kind == pointer.Drag:
			cx, cy := ctrl.CursorPos()
			dx := cx - u.dragStart.cursorX
			dy := cy - u.dragStart.cursorY
			newX := u.dragStart.winX + dx
			newY := u.dragStart.winY + dy
			// If already docked, constrain movement along the docked edge.
			u.constrainDragged(ctrl, &newX, &newY)
			ctrl.MoveWindow(newX, newY, 0, 0)
		case ev.Kind == pointer.Release:
			u.app.State.Dock.SetDragging(false)
			u.tryDock(ctrl)
		}
	}
}

// constrainDragged adjusts newX/newY so the window sticks to the docked edge
// while allowing free movement along it. If the window is dragged far from the
// edge, it undocks.
func (u *ListUI) constrainDragged(ctrl platformCtrl, newX, newY *int) {
	dock := u.app.State.Dock.Snapshot()
	if !dock.Docked {
		return
	}
	const undockThreshold = 20 // px from edge to break free
	wx2, wy2, ww2, wh2 := dock.WorkX, dock.WorkY, dock.WorkW, dock.WorkH
	switch dock.Edge {
	case store.EdgeTop:
		if abs(*newY-wy2) > undockThreshold {
			u.app.State.Dock.Reset()
			return
		}
		*newY = wy2
		u.app.State.Dock.UpdateDockPos(*newX, *newY)
	case store.EdgeBottom:
		if abs((*newY+dock.WinH)-(wy2+wh2)) > undockThreshold {
			u.app.State.Dock.Reset()
			return
		}
		*newY = wy2 + wh2 - dock.WinH
		u.app.State.Dock.UpdateDockPos(*newX, *newY)
	case store.EdgeLeft:
		if abs(*newX-wx2) > undockThreshold {
			u.app.State.Dock.Reset()
			return
		}
		*newX = wx2
		u.app.State.Dock.UpdateDockPos(*newX, *newY)
	case store.EdgeRight:
		if abs((*newX+dock.WinW)-(wx2+ww2)) > undockThreshold {
			u.app.State.Dock.Reset()
			return
		}
		*newX = wx2 + ww2 - dock.WinW
		u.app.State.Dock.UpdateDockPos(*newX, *newY)
	}
}

// tryDock checks the window position against screen edges and snaps if close.
// Corners prioritise Top/Bottom over Left/Right. The window keeps its current
// along-edge position (not forced to centre).
func (u *ListUI) tryDock(ctrl platformCtrl) {
	const threshold = 30 // pixels from edge to trigger snap
	wx, wy, ww, wh := ctrl.WindowGeometry()
	workX, workY, workW, workH := ctrl.WorkArea()
	if ww == 0 || workW == 0 {
		return
	}

	// Distance from each edge of the work area.
	nearTop := abs(wy-workY) < threshold
	nearBottom := abs((wy+wh)-(workY+workH)) < threshold
	nearLeft := abs(wx-workX) < threshold
	nearRight := abs((wx+ww)-(workX+workW)) < threshold

	var edge store.Edge
	switch {
	case nearTop:
		edge = store.EdgeTop
	case nearBottom:
		edge = store.EdgeBottom
	case nearLeft:
		edge = store.EdgeLeft
	case nearRight:
		edge = store.EdgeRight
	default:
		// Not near any edge — ensure undocked.
		u.app.State.Dock.Reset()
		return
	}

	// Snap: clamp to the edge but keep the current along-edge position.
	var dockX, dockY int
	switch edge {
	case store.EdgeTop:
		dockX = wx
		dockY = workY
	case store.EdgeBottom:
		dockX = wx
		dockY = workY + workH - wh
	case store.EdgeLeft:
		dockX = workX
		dockY = wy
	case store.EdgeRight:
		dockX = workX + workW - ww
		dockY = wy
	}

	ctrl.MoveWindow(dockX, dockY, ww, wh)
	u.app.State.Dock.SetDock(edge, dockX, dockY, ww, wh, workX, workY, workW, workH)
}

// abs returns the absolute value of x.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// platformCtrl is the subset of platform.Controller needed for dragging/docking.
type platformCtrl interface {
	WindowGeometry() (x, y, w, h int)
	MoveWindow(x, y, w, h int)
	WorkArea() (x, y, w, h int)
	CursorPos() (x, y int)
}

// table renders the header row plus all item rows in a scrollable list.
func (u *ListUI) table(gtx layout.Context, items []client.Todo, th *material.Theme) layout.Dimensions {
	return u.list.Layout(gtx, len(items)+1, func(gtx layout.Context, index int) layout.Dimensions {
		// Each list element is the row body stacked over a 1dp separator, so the
		// separator's height is counted and doesn't overlap the next row.
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if index == 0 {
					return u.headerRow(gtx, th)
				}
				todo := items[index-1]
				return u.row(gtx, todo, &u.rows[index-1], th)
			}),
			layout.Rigid(separator),
		)
	})
}

func (u *ListUI) headerRow(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return insetRow(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return material.Clickable(gtx, &u.headerTitle, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Label(th, unit.Sp(11), i18n.T("list.colTitle")+u.sortMarker("title"))
					lbl.Font.Weight = font.SemiBold
					lbl.Color = textSecondary
					return lbl.Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return colWidth(gtx, colPriorityW, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Label(th, unit.Sp(11), i18n.T("list.colPriority"))
					lbl.Font.Weight = font.SemiBold
					lbl.Color = textSecondary
					return lbl.Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return colWidth(gtx, colStatusW, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Label(th, unit.Sp(11), i18n.T("list.colStatus"))
					lbl.Font.Weight = font.SemiBold
					lbl.Color = textSecondary
					return lbl.Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return colWidth(gtx, colActionW, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Label(th, unit.Sp(11), i18n.T("list.colAction"))
					lbl.Font.Weight = font.SemiBold
					lbl.Color = textSecondary
					return lbl.Layout(gtx)
				})
			}),
		)
	})
}

// row renders one todo line, wrapped in a clickable for opening the detail.
func (u *ListUI) row(gtx layout.Context, todo client.Todo, row *rowWidgets, th *material.Theme) layout.Dimensions {
	// Highlight the row if its detail is currently open in the side window.
	selected := u.app.SideWin.IsDetailMode() && u.app.isSelected(todo.ID)
	return material.Clickable(gtx, &row.row, func(gtx layout.Context) layout.Dimensions {
		if selected {
			fillRectMax(gtx, bgRowSelect)
		}
		return insetRow(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				// Title + code.
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return titleCell(gtx, th, todo)
				}),
				// Priority.
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return colWidth(gtx, colPriorityW, func(gtx layout.Context) layout.Dimensions {
						return priorityCell(gtx, th, todo.Priority)
					})
				}),
				// Status.
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return colWidth(gtx, colStatusW, func(gtx layout.Context) layout.Dimensions {
						s := material.Label(th, unit.Sp(12), StatusLabel(todo.Status))
						s.Color = statusColor(todo.Status)
						return s.Layout(gtx)
					})
				}),
				// Action button.
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return colWidth(gtx, colActionW, func(gtx layout.Context) layout.Dimensions {
						return u.actionButton(gtx, todo, row, th)
					})
				}),
			)
		})
	})
}

// actionButton renders the start/complete icon button, or nothing for completed.
func (u *ListUI) actionButton(gtx layout.Context, todo client.Todo, row *rowWidgets, th *material.Theme) layout.Dimensions {
	switch todo.Status {
	case "open":
		return iconButton(gtx, th, &row.action, IconPlay, false)
	case "in_progress":
		return iconButton(gtx, th, &row.action, IconCheck, false)
	default:
		return layout.Dimensions{}
	}
}

func (u *ListUI) footer(gtx layout.Context, th *material.Theme, total int64, shown int, loading bool) layout.Dimensions {
	label := i18n.T("common.items", "count", total)
	if loading {
		label += i18n.T("list.refreshing")
	}
	return layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(8), Left: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		c := material.Caption(th, label)
		c.Color = textMuted
		return c.Layout(gtx)
	})
}

// --- click handling --------------------------------------------------------

func (u *ListUI) handleTopBarClicks(gtx layout.Context, w *app.Window) {
	if u.app.IsModal() {
		return
	}
	for u.headerTitle.Clicked(gtx) {
		u.toggleSort("title")
	}
	for u.createBtn.Clicked(gtx) {
		u.app.OpenCreate()
	}
	for u.refreshBtn.Clicked(gtx) {
		u.RequestRefresh()
	}
	for u.topMostBtn.Clicked(gtx) {
		u.app.SetTopMost(!u.app.isTopMost())
	}
	for u.lockBtn.Clicked(gtx) {
		u.app.SetLock(!u.app.isLocked())
	}
	for u.manageBtn.Clicked(gtx) {
		u.app.OpenManage()
	}
	for u.closeBtn.Clicked(gtx) {
		go u.promptCloseAction(w)
	}
}

// promptCloseAction opens the close-confirmation dialog (minimize / close /
// cancel). It runs the dialog's event loop on a dedicated goroutine and waits
// for the user's choice on a channel. While the dialog is open the main window
// is modal (interaction blocked via app.IsModal).
func (u *ListUI) promptCloseAction(mainWin *app.Window) {
	u.app.enterModal()
	resultCh := ConfirmCloseDialog(u.app)
	go func() {
		result := <-resultCh
		u.app.exitModal()
		switch result {
		case "minimize":
			if u.app.Platform != nil {
				u.app.Platform.Minimize()
			}
		case "close":
			mainWin.Perform(system.ActionClose)
		}
	}()
}

func (u *ListUI) handleRowClicks(gtx layout.Context, items []client.Todo) {
	if u.app.IsModal() {
		return
	}
	for i := range items {
		if i >= len(u.rows) {
			break
		}
		row := &u.rows[i]
		// Drain the action button first. If it fired this frame, skip the row
		// click to avoid both opening the detail and advancing the status.
		actionClicked := false
		for row.action.Clicked(gtx) {
			actionClicked = true
			u.doStatusAction(items[i])
		}
		if !actionClicked {
			for row.row.Clicked(gtx) {
				u.openDetail(items[i].ID)
			}
		}
	}
}

func (u *ListUI) openDetail(id uint) {
	u.app.OpenDetail(id)
}

// doStatusAction advances the todo to the next status.
func (u *ListUI) doStatusAction(todo client.Todo) {
	cl := u.app.Client()
	if cl == nil {
		return
	}
	idStr := fmt.Sprintf("%d", todo.ID)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var err error
		switch todo.Status {
		case "open":
			_, err = cl.StartTodo(ctx, idStr)
		case "in_progress":
			_, err = cl.CompleteTodo(ctx, idStr, false)
		}
		if err != nil {
			if _, ok := client.IsConflict(err); ok {
				u.app.State.SetMessage(i18n.T("list.blocked"))
			} else {
				u.app.State.SetMessage(i18n.T("list.actionFailed") + err.Error())
			}
		} else {
			u.app.State.SetMessage("")
		}
		u.RequestRefresh()
	}()
}

// toggleSort cycles the sort column/direction.
func (u *ListUI) toggleSort(col string) {
	u.app.State.Lock()
	f := &u.app.State.Config.Filters
	if f.SortBy == col {
		if f.SortOrder == "asc" {
			f.SortOrder = "desc"
		} else {
			f.SortOrder = "asc"
		}
	} else {
		f.SortBy = col
		f.SortOrder = "asc"
	}
	u.app.State.Unlock()
	u.RequestRefresh()
}

func (u *ListUI) sortMarker(col string) string {
	u.app.State.Lock()
	defer u.app.State.Unlock()
	if u.app.State.Config.Filters.SortBy != col {
		return ""
	}
	if u.app.State.Config.Filters.SortOrder == "asc" {
		return " ↑"
	}
	return " ↓"
}

// RequestRefresh reloads the list using the current filters.
func (u *ListUI) RequestRefresh() {
	cl := u.app.Client()
	if cl == nil {
		return
	}
	query := u.app.buildListQuery()
	u.app.Todos.Refresh(context.Background(), cl, query, func() {
		if u.app.Invalidate != nil {
			u.app.Invalidate()
		}
	})
}

// --- shared UI helpers -----------------------------------------------------

func smallButton(gtx layout.Context, th *material.Theme, c *widget.Clickable, label string) layout.Dimensions {
	btn := material.Button(th, c, label)
	styleButton(&btn)
	return uniformButton(gtx, &btn)
}

func titleCell(gtx layout.Context, th *material.Theme, todo client.Todo) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			code := material.Caption(th, DisplayCode(todo.Category, todo.Code))
			code.Color = textMuted
			return code.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			t := material.Label(th, unit.Sp(13), todo.Title)
			t.Color = titleColor(todo)
			return t.Layout(gtx)
		}),
	)
}

func titleColor(todo client.Todo) color.NRGBA {
	if todo.Status == "completed" {
		return textDisabled
	}
	return textPrimary
}

func priorityCell(gtx layout.Context, th *material.Theme, priority string) layout.Dimensions {
	pri := strings.ToUpper(priority)
	lbl := material.Label(th, unit.Sp(11), pri)
	lbl.Color = PriorityColor(priority)
	switch pri {
	case "P0":
		lbl.Font.Weight = font.Black
	case "P1":
		lbl.Font.Weight = font.Bold
	case "P2":
		lbl.Font.Weight = font.Medium
	default:
		lbl.Font.Weight = font.Normal
	}
	return verticalCenter(gtx, lbl.Layout)
}

func statusColor(status string) color.NRGBA {
	switch status {
	case "completed":
		return textDisabled
	case "in_progress":
		return textPrimary
	default:
		return textSecondary
	}
}
