package ui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op/clip"
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
	closeBtn   widget.Clickable

	list layout.List

	// Per-row clickables, indexed by row. Grown as needed each frame.
	rows []rowWidgets

	// Sort column header clickables.
	headerTitle widget.Clickable

	// firstLoad tracks whether the initial fetch has been triggered.
	firstLoad bool
}

type rowWidgets struct {
	row     widget.Clickable // whole row -> open detail
	action  widget.Clickable // start / complete button
}

func NewListUI(a *App) *ListUI {
	return &ListUI{app: a, list: layout.List{Axis: layout.Vertical}}
}

func (u *ListUI) Layout(gtx layout.Context, w *app.Window) layout.Dimensions {
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

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(u.topBar),
		layout.Rigid(separator),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			switch {
			case err != nil:
				return centeredText(gtx, u.app.Theme, i18n.T("common.error")+err.Error(), textMuted)
			case loading && len(items) == 0:
				return centeredText(gtx, u.app.Theme, i18n.T("common.loading"), textMuted)
			case len(items) == 0:
				return centeredText(gtx, u.app.Theme, i18n.T("list.noTodos"), textMuted)
			default:
				return u.table(gtx, items)
			}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.footer(gtx, total, len(items), loading)
		}),
	)
}

// topBar renders the title and the pin/lock/manage icon buttons. The title text
// is the window drag handle for the frameless window; the drag action is scoped
// to a small explicit region so the buttons on the right keep receiving clicks.
func (u *ListUI) topBar(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(12), Right: unit.Dp(8)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				// Draggable title: lay out the text, then register ActionMove over
				// its exact bounds via a clip so it cannot swallow button clicks.
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					t := material.Body1(u.app.Theme, i18n.T("list.title"))
					t.TextSize = unit.Sp(16)
					t.Font.Weight = font.SemiBold
					t.Color = textPrimary
					dims := t.Layout(gtx)
					// Scope the drag handle to the title's own bounding box.
					r := image.Rect(0, 0, dims.Size.X, dims.Size.Y)
					defer clip.Rect(r).Push(gtx.Ops).Pop()
					system.ActionInputOp(system.ActionMove).Add(gtx.Ops)
					return dims
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return iconButton(gtx, u.app.Theme, &u.refreshBtn, IconRefresh, false) }),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return iconButton(gtx, u.app.Theme, &u.topMostBtn, IconPin, u.app.isTopMost()) }),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return iconButton(gtx, u.app.Theme, &u.lockBtn, IconLock, u.app.isLocked()) }),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return iconButton(gtx, u.app.Theme, &u.manageBtn, IconSettings, false) }),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return iconButton(gtx, u.app.Theme, &u.closeBtn, IconClose, false) }),
			)
		},
	)
}

// table renders the header row plus all item rows in a scrollable list.
func (u *ListUI) table(gtx layout.Context, items []client.Todo) layout.Dimensions {
	return u.list.Layout(gtx, len(items)+1, func(gtx layout.Context, index int) layout.Dimensions {
		// Each list element is the row body stacked over a 1dp separator, so the
		// separator's height is counted and doesn't overlap the next row.
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if index == 0 {
					return u.headerRow(gtx)
				}
				todo := items[index-1]
				return u.row(gtx, todo, &u.rows[index-1])
			}),
			layout.Rigid(separator),
		)
	})
}

func (u *ListUI) headerRow(gtx layout.Context) layout.Dimensions {
	return insetRow(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return material.Clickable(gtx, &u.headerTitle, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Label(u.app.Theme, unit.Sp(11), i18n.T("list.colTitle")+u.sortMarker("title"))
					lbl.Font.Weight = font.SemiBold
					lbl.Color = textSecondary
					return lbl.Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return colWidth(gtx, 26, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Label(u.app.Theme, unit.Sp(11), i18n.T("list.colPriority"))
					lbl.Font.Weight = font.SemiBold
					lbl.Color = textSecondary
					return lbl.Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return colWidth(gtx, 78, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Label(u.app.Theme, unit.Sp(11), i18n.T("list.colStatus"))
					lbl.Font.Weight = font.SemiBold
					lbl.Color = textSecondary
					return lbl.Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return colWidth(gtx, 52, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{}
				})
			}),
		)
	})
}

// row renders one todo line, wrapped in a clickable for opening the detail.
func (u *ListUI) row(gtx layout.Context, todo client.Todo, row *rowWidgets) layout.Dimensions {
	return material.Clickable(gtx, &row.row, func(gtx layout.Context) layout.Dimensions {
		return insetRow(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				// Title + code.
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return titleCell(gtx, u.app.Theme, todo)
				}),
				// Priority.
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return colWidth(gtx, 26, func(gtx layout.Context) layout.Dimensions {
						return priorityCell(gtx, u.app.Theme, todo.Priority)
					})
				}),
				// Status.
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return colWidth(gtx, 78, func(gtx layout.Context) layout.Dimensions {
						s := material.Label(u.app.Theme, unit.Sp(12), StatusLabel(todo.Status))
						s.Color = statusColor(todo.Status)
						return s.Layout(gtx)
					})
				}),
				// Action button.
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return colWidth(gtx, 52, func(gtx layout.Context) layout.Dimensions {
						return u.actionButton(gtx, todo, row)
					})
				}),
			)
		})
	})
}

// actionButton renders the start/complete icon button, or nothing for completed.
func (u *ListUI) actionButton(gtx layout.Context, todo client.Todo, row *rowWidgets) layout.Dimensions {
	switch todo.Status {
	case "open":
		return iconButton(gtx, u.app.Theme, &row.action, IconPlay, false)
	case "in_progress":
		return iconButton(gtx, u.app.Theme, &row.action, IconCheck, false)
	default:
		return layout.Dimensions{}
	}
}

func (u *ListUI) footer(gtx layout.Context, total int64, shown int, loading bool) layout.Dimensions {
	label := i18n.T("common.items", "count", total)
	if loading {
		label += i18n.T("list.refreshing")
	}
	return layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(8), Left: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		c := material.Caption(u.app.Theme, label)
		c.Color = textMuted
		return c.Layout(gtx)
	})
}

// --- click handling --------------------------------------------------------

func (u *ListUI) handleTopBarClicks(gtx layout.Context, w *app.Window) {
	for u.headerTitle.Clicked(gtx) {
		u.toggleSort("title")
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
		u.app.nav().goTo(store.PageManage)
	}
	for u.closeBtn.Clicked(gtx) {
		w.Perform(system.ActionClose)
	}
}

func (u *ListUI) handleRowClicks(gtx layout.Context, items []client.Todo) {
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
	u.app.State.Lock()
	u.app.State.SelectedID = id
	u.app.State.Page = store.PageDetail
	u.app.State.Unlock()
	u.app.Todos.ResetDetail()
	u.app.Detail.Load()
	if u.app.Invalidate != nil {
		u.app.Invalidate()
	}
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
	btn.TextSize = unit.Sp(11)
	btn.Inset = layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(3), Left: unit.Dp(6), Right: unit.Dp(6)}
	btn.Background = textPrimary
	btn.Color = bgPage
	return btn.Layout(gtx)
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
