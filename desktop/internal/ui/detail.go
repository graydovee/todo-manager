package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/graydovee/todo-manager/desktop/internal/client"
	"github.com/graydovee/todo-manager/desktop/internal/i18n"
	"github.com/graydovee/todo-manager/desktop/internal/store"
)

// DetailUI shows a single todo with its relations and comments, and supports
// editing and status transitions.
type DetailUI struct {
	app *App
	// th is the theme used to render this instance. It is normally the app's
	// main theme, but the side window injects the side theme so the two
	// windows don't share a text.Shaper (which is not concurrency-safe).
	th *material.Theme

	backBtn     widget.Clickable
	editBtn     widget.Clickable
	saveBtn     widget.Clickable
	cancelBtn   widget.Clickable
	startBtn    widget.Clickable
	completeBtn widget.Clickable
	reopenBtn   widget.Clickable
	deleteBtn   widget.Clickable
	cascadeBtn  widget.Clickable // for conflict modal

	// Edit fields.
	edTitle       widget.Editor
	edDescription widget.Editor
	edPriority    widget.Editor
	edDueAt       widget.Editor
	edTags        widget.Editor

	// Comments.
	edComment   widget.Editor
	commentBtn  widget.Clickable
	commentRows []commentRow

	// Edit mode.
	editing bool
	// Pending conflict after a complete/reopen 409.
	pendingConflict *client.ConflictResponse

	// onBack, if set, overrides the default back-button behaviour (navigate to
	// PageList). The detail side window sets this to close itself instead.
	onBack func()

	// hideHeader, when true, suppresses the header row (back button + title +
	// edit/save). Used by the detail side window which draws its own top bar.
	hideHeader bool
}

type commentRow struct {
	del widget.Clickable
}

func NewDetailUI(a *App) *DetailUI {
	d := &DetailUI{app: a, th: a.Theme}
	d.edTitle.SingleLine = true
	d.edTitle.Submit = true
	d.edPriority.SingleLine = true
	d.edDueAt.SingleLine = true
	d.edTags.SingleLine = true
	d.edComment.SingleLine = true
	d.edComment.Submit = true
	return d
}

// Load triggers a detail fetch for the currently selected id.
func (d *DetailUI) Load() {
	cl := d.app.Client()
	if cl == nil {
		return
	}
	d.app.State.Lock()
	id := d.app.State.SelectedID
	d.app.State.Unlock()
	if id == 0 {
		return
	}
	d.app.Todos.LoadDetail(context.Background(), cl, fmt.Sprintf("%d", id), func() {
		if d.app.Invalidate != nil {
			d.app.Invalidate()
		}
	})
}

func (d *DetailUI) Layout(gtx layout.Context, w *app.Window) layout.Dimensions {
	d.handleClicks(gtx)

	detail, loading, err := d.app.Todos.DetailSnapshot()

	// Conflict modal takes over if present.
	if d.pendingConflict != nil {
		return d.conflictModal(gtx)
	}

	children := make([]layout.FlexChild, 0, 3)
	if !d.hideHeader {
		children = append(children,
			layout.Rigid(d.header),
			layout.Rigid(separator),
		)
	}
	children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		if err != nil {
			return centeredText(gtx, d.th, i18n.T("common.error")+err.Error(), textMuted)
		}
		if loading || detail == nil {
			return centeredText(gtx, d.th, i18n.T("common.loading"), textMuted)
		}
		list := &layout.List{Axis: layout.Vertical}
		return list.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
			return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if d.editing {
					return d.editForm(gtx, detail)
				}
				return d.viewBody(gtx, detail)
			})
		})
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (d *DetailUI) header(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return iconButton(gtx, d.th, &d.backBtn, IconBack, false) }),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					t := material.Body1(d.th, i18n.T("detail.title"))
					t.Font.Weight = font.SemiBold
					t.Color = textPrimary
					return t.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if d.editing {
						return smallButton(gtx, d.th, &d.saveBtn, i18n.T("common.save"))
					}
					return smallButton(gtx, d.th, &d.editBtn, i18n.T("common.edit"))
				}),
			)
		},
	)
}

func (d *DetailUI) viewBody(gtx layout.Context, detail *client.TodoDetail) layout.Dimensions {
	todo := detail.Todo
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Code + title.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return codeTitle(gtx, d.th, todo)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		// Action buttons row.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.actionRow(gtx, todo)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		// Description.
		layout.Rigid(d.descriptionBlock(todo)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		// Metadata.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.metaBlock(gtx, detail)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		// Relations.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.relationsBlock(gtx, detail)
		}),
		// Comments.
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.commentsBlock(gtx, detail)
		}),
	)
}

func (d *DetailUI) actionRow(gtx layout.Context, todo client.Todo) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			switch todo.Status {
			case "open":
				return smallButton(gtx, d.th, &d.startBtn, i18n.T("detail.start"))
			case "in_progress":
				return smallButton(gtx, d.th, &d.completeBtn, i18n.T("detail.complete"))
			case "completed":
				return smallButton(gtx, d.th, &d.reopenBtn, i18n.T("detail.reopen"))
			}
			return layout.Dimensions{}
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return smallButton(gtx, d.th, &d.deleteBtn, i18n.T("common.delete"))
		}),
	)
}

func (d *DetailUI) descriptionBlock(todo client.Todo) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if strings.TrimSpace(todo.Description) == "" {
			return layout.Dimensions{}
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(sectionLabel(d.th, i18n.T("detail.description"))),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				body := material.Body2(d.th, todo.Description)
				body.Color = textPrimary
				return body.Layout(gtx)
			}),
		)
	}
}

func (d *DetailUI) metaBlock(gtx layout.Context, detail *client.TodoDetail) layout.Dimensions {
	todo := detail.Todo
	rows := [][2]string{
		{i18n.T("detail.due"), dashIfEmpty(formatTimePtr(todo.DueAt))},
		{i18n.T("detail.created"), formatTime(todo.CreatedAt)},
		{i18n.T("detail.updated"), formatTime(todo.UpdatedAt)},
		{i18n.T("detail.priority"), strings.ToUpper(todo.Priority)},
		{i18n.T("detail.tags"), dashIfEmpty(strings.Join(todo.Tags, ", "))},
	}
	list := &layout.List{Axis: layout.Vertical}
	return list.Layout(gtx, len(rows), func(gtx layout.Context, i int) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return colWidth(gtx, 70, func(gtx layout.Context) layout.Dimensions {
					c := material.Caption(d.th, rows[i][0])
					c.Color = textSecondary
					return c.Layout(gtx)
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				c := material.Caption(d.th, rows[i][1])
				c.Color = textPrimary
				return c.Layout(gtx)
			}),
		)
	})
}

func (d *DetailUI) relationsBlock(gtx layout.Context, detail *client.TodoDetail) layout.Dimensions {
	hasDeps := len(detail.DependsOn) > 0 || len(detail.DependedBy) > 0
	hasDups := detail.DuplicateOf != nil || len(detail.Duplicates) > 0
	if !hasDeps && !hasDups {
		return layout.Dimensions{}
	}
	var sections []layout.Widget
	if hasDeps {
		sections = append(sections, d.summaryList(i18n.T("detail.prereq"), detail.DependsOn))
		sections = append(sections, d.summaryList(i18n.T("detail.dependents"), detail.DependedBy))
	}
	if hasDups {
		if detail.DuplicateOf != nil {
			sections = append(sections, d.summaryList(i18n.T("detail.duplicateOf"), []client.TodoSummary{*detail.DuplicateOf}))
		}
		if len(detail.Duplicates) > 0 {
			sections = append(sections, d.summaryList(i18n.T("detail.duplicates"), detail.Duplicates))
		}
	}
	list := &layout.List{Axis: layout.Vertical}
	return list.Layout(gtx, len(sections), func(gtx layout.Context, i int) layout.Dimensions {
		return sections[i](gtx)
	})
}

func (d *DetailUI) summaryList(label string, items []client.TodoSummary) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(sectionLabel(d.th, label)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				list := &layout.List{Axis: layout.Vertical}
				return list.Layout(gtx, len(items), func(gtx layout.Context, i int) layout.Dimensions {
					it := items[i]
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							c := material.Caption(d.th, DisplayCode(it.Category, it.Code))
							c.Color = textMuted
							return c.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							c := material.Caption(d.th, it.Title)
							c.Color = textPrimary
							return c.Layout(gtx)
						}),
					)
				})
			}),
		)
	}
}

func (d *DetailUI) commentsBlock(gtx layout.Context, detail *client.TodoDetail) layout.Dimensions {
	comments := d.app.Todos.CommentsSnapshot()
	// Keep the per-comment delete clickables sized to the comment count.
	if cap(d.commentRows) < len(comments) {
		d.commentRows = make([]commentRow, len(comments))
	} else {
		d.commentRows = d.commentRows[:len(comments)]
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(sectionLabel(d.th, i18n.T("detail.comments"))),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(comments) == 0 {
				c := material.Caption(d.th, i18n.T("detail.noComments"))
				c.Color = textMuted
				return c.Layout(gtx)
			}
			list := &layout.List{Axis: layout.Vertical}
			return list.Layout(gtx, len(comments), func(gtx layout.Context, i int) layout.Dimensions {
				cm := comments[i]
				if i >= len(d.commentRows) {
					return layout.Dimensions{}
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								c := material.Caption(d.th, cm.Content)
								c.Color = textPrimary
								return c.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								c := material.Caption(d.th, formatTime(cm.CreatedAt))
								c.Color = textMuted
								return c.Layout(gtx)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return smallButton(gtx, d.th, &d.commentRows[i].del, i18n.T("detail.del"))
					}),
				)
			})
		}),
		// New comment input.
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					ed := material.Editor(d.th, &d.edComment, "Add a comment…")
					ed.Color = textPrimary
					return ed.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return smallButton(gtx, d.th, &d.commentBtn, i18n.T("detail.send"))
				}),
			)
		}),
	)
}

func (d *DetailUI) editForm(gtx layout.Context, detail *client.TodoDetail) layout.Dimensions {
	todo := detail.Todo
	// Initialise editors once when entering edit mode.
	if d.edTitle.Text() == "" {
		d.edTitle.SetText(todo.Title)
		d.edDescription.SetText(todo.Description)
		d.edPriority.SetText(strings.ToUpper(todo.Priority))
		if todo.DueAt != nil {
			d.edDueAt.SetText(*todo.DueAt)
		}
		d.edTags.SetText(strings.Join(todo.Tags, ","))
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return labeledEditor(gtx, d.th, &d.edTitle, i18n.T("list.colTitle"), "")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return labeledEditor(gtx, d.th, &d.edDescription, i18n.T("detail.description"), "")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return labeledEditor(gtx, d.th, &d.edPriority, i18n.T("detail.priorityHint"), "")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return labeledEditor(gtx, d.th, &d.edDueAt, i18n.T("detail.dueHint"), "")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return labeledEditor(gtx, d.th, &d.edTags, i18n.T("detail.tagsHint"), "")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			c := material.Caption(d.th, i18n.T("detail.categoryNote", "category", todo.Category))
			c.Color = textMuted
			return c.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
	}
	// Save/Cancel buttons — only in standalone (non-side-window) mode. The side
	// window renders these in its top bar; rendering the same Clickable twice in
	// one frame breaks click detection.
	if !d.hideHeader {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return smallButton(gtx, d.th, &d.saveBtn, i18n.T("common.save"))
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return smallButton(gtx, d.th, &d.cancelBtn, i18n.T("common.cancel"))
				}),
			)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// conflictModal shows pending dependency blockers and offers a cascade.
func (d *DetailUI) conflictModal(gtx layout.Context) layout.Dimensions {
	cf := d.pendingConflict
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					t := material.H6(d.th, i18n.T("detail.blocked"))
					t.Color = textPrimary
					return t.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					blockers := cf.PendingDependencies
					if len(blockers) == 0 {
						blockers = cf.CompletedDependents
					}
					list := &layout.List{Axis: layout.Vertical}
					return list.Layout(gtx, len(blockers), func(gtx layout.Context, i int) layout.Dimensions {
						b := blockers[i]
						c := material.Body2(d.th, fmt.Sprintf("%s %s", DisplayCode(b.Category, b.Code), b.Title))
						c.Color = textSecondary
						return c.Layout(gtx)
					})
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return smallButton(gtx, d.th, &d.cascadeBtn, i18n.T("detail.cascade"))
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return smallButton(gtx, d.th, &d.cancelBtn, i18n.T("detail.dismiss"))
						}),
					)
				}),
			)
		})
	})
}

// --- click handling --------------------------------------------------------

func (d *DetailUI) handleClicks(gtx layout.Context) {
	for d.backBtn.Clicked(gtx) {
		if d.onBack != nil {
			d.onBack()
		} else {
			d.app.nav().goTo(store.PageList)
		}
	}
	for d.editBtn.Clicked(gtx) {
		d.editing = true
		d.resetEditors()
	}
	for d.cancelBtn.Clicked(gtx) {
		d.editing = false
		d.pendingConflict = nil
		d.resetEditors()
	}
	for d.saveBtn.Clicked(gtx) {
		if d.editing {
			d.saveEdit()
		}
	}
	for d.startBtn.Clicked(gtx) {
		d.transition("start", false)
	}
	for d.completeBtn.Clicked(gtx) {
		d.transition("complete", false)
	}
	for d.reopenBtn.Clicked(gtx) {
		d.transition("reopen", false)
	}
	for d.deleteBtn.Clicked(gtx) {
		d.deleteTodo()
	}
	for d.cascadeBtn.Clicked(gtx) {
		if d.pendingConflict != nil {
			// Determine which transition is pending from the conflict shape.
			if len(d.pendingConflict.PendingDependencies) > 0 {
				d.transition("complete", true)
			} else {
				d.transition("reopen", true)
			}
			d.pendingConflict = nil
		}
	}
	for d.commentBtn.Clicked(gtx) {
		d.addComment()
	}
	comments := d.app.Todos.CommentsSnapshot()
	for i := range d.commentRows {
		if i < len(comments) && d.commentRows[i].del.Clicked(gtx) {
			d.deleteComment(comments[i].ID)
		}
	}
}

func (d *DetailUI) resetEditors() {
	d.edTitle.SetText("")
	d.edDescription.SetText("")
	d.edPriority.SetText("")
	d.edDueAt.SetText("")
	d.edTags.SetText("")
}

// saveEdit persists the edited fields via PATCH.
func (d *DetailUI) saveEdit() {
	cl := d.app.Client()
	if cl == nil {
		return
	}
	id := d.selectedIDStr()
	if id == "" {
		return
	}
	body := map[string]any{
		"title":       d.edTitle.Text(),
		"description": d.edDescription.Text(),
		"priority":    normalizePriority(d.edPriority.Text()),
		"tags":        parseTags(d.edTags.Text()),
	}
	if due := strings.TrimSpace(d.edDueAt.Text()); due != "" {
		body["due_at"] = due
	} else {
		body["due_at"] = nil
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := cl.UpdateTodo(ctx, id, body); err != nil {
			d.app.State.SetMessage(i18n.T("detail.saveFailed") + err.Error())
		} else {
			d.app.State.SetMessage(i18n.T("detail.saved"))
		}
		d.editing = false
		d.resetEditors()
		d.Load()
		d.app.List.RequestRefresh()
	}()
}

// transition performs a status transition, surfacing conflicts as a modal.
func (d *DetailUI) transition(action string, cascade bool) {
	cl := d.app.Client()
	if cl == nil {
		return
	}
	id := d.selectedIDStr()
	if id == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var err error
		switch action {
		case "start":
			_, err = cl.StartTodo(ctx, id)
		case "complete":
			_, err = cl.CompleteTodo(ctx, id, cascade)
		case "reopen":
			_, err = cl.ReopenTodo(ctx, id, cascade)
		}
		if err != nil {
			if cf, ok := client.IsConflict(err); ok && !cascade {
				d.pendingConflict = cf
			} else {
				d.app.State.SetMessage(i18n.T("detail.failed") + err.Error())
			}
		} else {
			d.app.State.SetMessage("")
		}
		d.Load()
		d.app.List.RequestRefresh()
	}()
}

func (d *DetailUI) deleteTodo() {
	cl := d.app.Client()
	if cl == nil {
		return
	}
	id := d.selectedIDStr()
	if id == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := cl.DeleteTodo(ctx, id); err != nil {
			d.app.State.SetMessage(i18n.T("detail.deleteFailed") + err.Error())
			return
		}
		d.app.nav().goTo(store.PageList)
		d.app.List.RequestRefresh()
	}()
}

func (d *DetailUI) addComment() {
	cl := d.app.Client()
	if cl == nil {
		return
	}
	content := strings.TrimSpace(d.edComment.Text())
	if content == "" {
		return
	}
	id := d.selectedIDStr()
	if id == "" {
		return
	}
	d.edComment.SetText("")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := cl.CreateComment(ctx, id, content); err != nil {
			d.app.State.SetMessage(i18n.T("detail.commentFailed") + err.Error())
		}
		d.Load()
	}()
}

func (d *DetailUI) deleteComment(commentID uint) {
	cl := d.app.Client()
	if cl == nil {
		return
	}
	id := d.selectedIDStr()
	if id == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := cl.DeleteComment(ctx, id, fmt.Sprintf("%d", commentID)); err != nil {
			d.app.State.SetMessage(i18n.T("detail.delCommentFail") + err.Error())
		}
		d.Load()
	}()
}

func (d *DetailUI) selectedIDStr() string {
	d.app.State.Lock()
	defer d.app.State.Unlock()
	if d.app.State.SelectedID == 0 {
		return ""
	}
	return fmt.Sprintf("%d", d.app.State.SelectedID)
}

// --- formatting helpers ----------------------------------------------------

func codeTitle(gtx layout.Context, th *material.Theme, todo client.Todo) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			c := material.Caption(th, DisplayCode(todo.Category, todo.Code)+"  ·  "+StatusLabel(todo.Status))
			c.Color = textMuted
			return c.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			t := material.H6(th, todo.Title)
			t.Color = textPrimary
			return t.Layout(gtx)
		}),
	)
}

// sectionLabel returns a layout.Widget rendering a small semi-bold heading.
func sectionLabel(th *material.Theme, label string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		c := material.Caption(th, label)
		c.Color = textSecondary
		c.Font.Weight = font.SemiBold
		return c.Layout(gtx)
	}
}

func dashIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func formatTime(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Format("2006-01-02 15:04")
}

func formatTimePtr(s *string) string {
	if s == nil {
		return ""
	}
	return formatTime(*s)
}

func normalizePriority(s string) string {
	p := strings.ToLower(strings.TrimSpace(s))
	switch p {
	case "p0", "p1", "p3":
		return p
	default:
		return "p2"
	}
}

func parseTags(s string) []string {
	parts := strings.Split(s, ",")
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(strings.ToLower(p))
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}
