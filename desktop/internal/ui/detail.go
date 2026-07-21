package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/graydovee/todo-manager/desktop/internal/client"
	"github.com/graydovee/todo-manager/desktop/internal/i18n"
	"github.com/graydovee/todo-manager/desktop/internal/store"
)

// DetailView renders the selected todo's full detail (relations, comments,
// editable form) inside the side panel. The view is rebuilt lazily from the
// store's detail snapshot.
type DetailView struct {
	app *App

	root   *fyne.Container
	scroll *container.Scroll

	// Read-only fields.
	codeTitle *widget.Label
	statusLbl *widget.Label

	// Action buttons.
	startBtn    *widget.Button
	completeBtn *widget.Button
	reopenBtn   *widget.Button
	blockBtn    *widget.Button

	// Metadata.
	descLabel   *widget.Label
	priorityLbl *widget.Label
	dueLbl      *widget.Label
	createdLbl  *widget.Label
	updatedLbl  *widget.Label
	tagsLabel   *widget.Label

	// Relations.
	prereqLabel    *widget.Label
	dependentsLbl  *widget.Label
	duplicateOfLbl *widget.Label
	duplicatesLbl  *widget.Label

	// Comments.
	commentEntry *widget.Entry
	sendBtn      *widget.Button
	commentBox   *fyne.Container

	// Edit form (toggled via the side-panel chrome Edit/Save button).
	editForm *fyne.Container

	titleEdit    *widget.Entry
	descEdit     *widget.Entry
	priorityEdit *widget.Entry
	tagsEdit     *widget.Entry
	dueEdit      *widget.Entry
}

func newDetailView(app *App) *DetailView {
	v := &DetailView{app: app}

	v.codeTitle = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	v.statusLbl = widget.NewLabel("")

	v.startBtn = widget.NewButton(i18n.T("detail.start"), nil)
	v.startBtn.Importance = widget.HighImportance
	v.completeBtn = widget.NewButton(i18n.T("detail.complete"), nil)
	v.completeBtn.Importance = widget.HighImportance
	v.reopenBtn = widget.NewButton(i18n.T("detail.reopen"), nil)
	v.reopenBtn.Importance = widget.HighImportance
	v.blockBtn = widget.NewButton(i18n.T("detail.blocked"), nil)
	v.blockBtn.Disable()

	v.descLabel = widget.NewLabel("")
	v.descLabel.Wrapping = fyne.TextWrapWord
	v.priorityLbl = widget.NewLabel("")
	v.dueLbl = widget.NewLabel("")
	v.createdLbl = widget.NewLabel("")
	v.updatedLbl = widget.NewLabel("")
	v.tagsLabel = widget.NewLabel("")

	v.prereqLabel = widget.NewLabel("")
	v.prereqLabel.Wrapping = fyne.TextWrapWord
	v.dependentsLbl = widget.NewLabel("")
	v.dependentsLbl.Wrapping = fyne.TextWrapWord
	v.duplicateOfLbl = widget.NewLabel("")
	v.duplicatesLbl = widget.NewLabel("")
	v.duplicatesLbl.Wrapping = fyne.TextWrapWord

	v.commentEntry = widget.NewMultiLineEntry()
	v.commentEntry.SetPlaceHolder(i18n.T("detail.commentHint"))
	v.commentEntry.SetMinRowsVisible(2)
	v.sendBtn = widget.NewButton(i18n.T("detail.send"), v.sendComment)
	v.sendBtn.Importance = widget.HighImportance
	v.commentBox = container.NewVBox()

	// Edit form.
	v.titleEdit = widget.NewEntry()
	v.titleEdit.SetPlaceHolder(i18n.T("manage.titleLabel"))
	v.descEdit = widget.NewMultiLineEntry()
	v.descEdit.SetPlaceHolder(i18n.T("manage.descLabel"))
	v.descEdit.SetMinRowsVisible(3)
	v.priorityEdit = widget.NewEntry()
	v.priorityEdit.SetPlaceHolder(i18n.T("detail.priorityHint"))
	v.tagsEdit = widget.NewEntry()
	v.tagsEdit.SetPlaceHolder(i18n.T("detail.tagsHint"))
	v.dueEdit = widget.NewEntry()
	v.dueEdit.SetPlaceHolder(i18n.T("detail.dueHint"))

	v.editForm = container.NewVBox(
		widget.NewLabel(i18n.T("manage.titleLabel")),
		v.titleEdit,
		widget.NewLabel(i18n.T("manage.descLabel")),
		v.descEdit,
		widget.NewLabel(i18n.T("detail.priorityHint")),
		v.priorityEdit,
		widget.NewLabel(i18n.T("detail.tagsHint")),
		v.tagsEdit,
		widget.NewLabel(i18n.T("detail.dueHint")),
		v.dueEdit,
	)
	v.editForm.Hide()

	actions := container.NewHBox(v.startBtn, v.completeBtn, v.reopenBtn, v.blockBtn)

	content := container.NewVBox(
		v.codeTitle,
		v.statusLbl,
		actions,
		widget.NewSeparator(),
		v.sectionLabel(i18n.T("detail.description")),
		v.descLabel,
		widget.NewSeparator(),
		container.NewGridWithColumns(2,
			container.NewVBox(v.sectionLabel(i18n.T("detail.priority")), v.priorityLbl),
			container.NewVBox(v.sectionLabel(i18n.T("detail.due")), v.dueLbl),
		),
		container.NewGridWithColumns(2,
			container.NewVBox(v.sectionLabel(i18n.T("detail.created")), v.createdLbl),
			container.NewVBox(v.sectionLabel(i18n.T("detail.updated")), v.updatedLbl),
		),
		v.sectionLabel(i18n.T("detail.tags")),
		v.tagsLabel,
		widget.NewSeparator(),
		v.sectionLabel(i18n.T("detail.prereq")),
		v.prereqLabel,
		v.sectionLabel(i18n.T("detail.dependents")),
		v.dependentsLbl,
		v.sectionLabel(i18n.T("detail.duplicateOf")),
		v.duplicateOfLbl,
		v.sectionLabel(i18n.T("detail.duplicates")),
		v.duplicatesLbl,
		widget.NewSeparator(),
		v.sectionLabel(i18n.T("detail.comments")),
		v.commentBox,
		container.NewBorder(nil, nil, nil, v.sendBtn, v.commentEntry),
		widget.NewSeparator(),
		v.editForm,
	)
	v.scroll = container.NewVScroll(content)
	v.root = container.NewPadded(v.scroll)
	return v
}

// Build returns the detail panel content (already wrapped in scroll).
func (v *DetailView) Build() fyne.CanvasObject {
	return v.root
}

// sectionLabel is a styled heading for a detail section.
func (v *DetailView) sectionLabel(text string) *widget.Label {
	return widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
}

// Refresh re-reads the detail snapshot and re-populates the widgets.
func (v *DetailView) Refresh() {
	detail, loading, err := v.app.Todos.DetailSnapshot()
	if loading || err != nil || detail == nil {
		v.codeTitle.SetText(i18n.T("common.loading"))
		return
	}
	t := detail.Todo
	v.codeTitle.SetText(fmt.Sprintf("%s  %s", formatDisplayCode(t.Category, t.Code), t.Title))
	v.statusLbl.SetText(statusLabel(t.Status))
	v.descLabel.SetText(orDash(t.Description))
	v.priorityLbl.SetText(priorityLabel(t.Priority))
	v.dueLbl.SetText(orDash(derefStr(t.DueAt)))
	v.createdLbl.SetText(formatTime(t.CreatedAt))
	v.updatedLbl.SetText(formatTime(t.UpdatedAt))
	v.tagsLabel.SetText(strings.Join(t.Tags, ", "))

	v.prereqLabel.SetText(summariesToText(detail.DependsOn))
	v.dependentsLbl.SetText(summariesToText(detail.DependedBy))
	if detail.DuplicateOf != nil {
		v.duplicateOfLbl.SetText(summaryToText(*detail.DuplicateOf))
	} else {
		v.duplicateOfLbl.SetText("-")
	}
	v.duplicatesLbl.SetText(summariesToText(detail.Duplicates))

	// Action buttons visibility.
	v.startBtn.Hidden = t.Status != statusOpen
	v.completeBtn.Hidden = t.Status == statusCompleted || t.Status == statusDuplicate
	v.reopenBtn.Hidden = t.Status != statusCompleted
	v.blockBtn.Hidden = true // shown only on conflict
	for _, b := range []*widget.Button{v.startBtn, v.completeBtn, v.reopenBtn, v.blockBtn} {
		b.Refresh()
	}

	// Hook action callbacks.
	v.startBtn.OnTapped = func() { v.action("start", false) }
	v.completeBtn.OnTapped = func() { v.action("complete", false) }
	v.reopenBtn.OnTapped = func() { v.action("reopen", false) }

	// Comments.
	v.commentBox.Objects = v.renderComments()
	v.commentBox.Refresh()
}

// renderComments builds a list of widgets for the cached comments.
func (v *DetailView) renderComments() []fyne.CanvasObject {
	comments := v.app.Todos.CommentsSnapshot()
	if len(comments) == 0 {
		return []fyne.CanvasObject{widget.NewLabel(i18n.T("detail.noComments"))}
	}
	out := make([]fyne.CanvasObject, 0, len(comments))
	for _, c := range comments {
		c := c
		body := widget.NewLabel(c.Content)
		body.Wrapping = fyne.TextWrapWord
		meta := widget.NewLabelWithStyle(
			fmt.Sprintf("%s", formatTime(c.CreatedAt)),
			fyne.TextAlignLeading, fyne.TextStyle{Italic: true},
		)
		del := widget.NewButton(i18n.T("detail.del"), func() { v.deleteComment(c.ID) })
		out = append(out, container.NewBorder(nil, nil, nil, del,
			container.NewVBox(meta, body)))
	}
	return out
}

// action runs a start/complete/reopen action with a 15s context.
func (v *DetailView) action(name string, cascade bool) {
	detail, _, _ := v.app.Todos.DetailSnapshot()
	if detail == nil {
		return
	}
	idStr := store.IDString(detail.Todo.ID)
	c := v.app.State.Client
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var err error
		switch name {
		case "start":
			_, err = c.StartTodo(ctx, idStr)
		case "complete":
			_, err = c.CompleteTodo(ctx, idStr, cascade)
		case "reopen":
			_, err = c.ReopenTodo(ctx, idStr, cascade)
		}
		if err != nil {
			if conf, ok := client.IsConflict(err); ok {
				fyne.Do(func() { v.showConflict(name, conf) })
				return
			}
			v.app.SetMessage(i18n.T("detail.failed") + err.Error())
			return
		}
		fyne.Do(func() {
			v.app.refreshList()
			v.Refresh()
		})
	}()
}

// showConflict displays a dialog offering cascade.
func (v *DetailView) showConflict(action string, conf *client.ConflictResponse) {
	msg := conf.Error
	if msg == "" {
		msg = i18n.T("list.blocked")
	}
	dialog.NewConfirm(i18n.T("detail.blocked"), msg, func(cascade bool) {
		if cascade {
			v.action(action, true)
		}
	}, v.app.Window).Show()
}

// IsEditing reports whether the inline edit form is currently visible.
func (v *DetailView) IsEditing() bool {
	return v.editForm != nil && v.editForm.Visible()
}

// enterEdit populates the edit form with the current detail and shows it.
func (v *DetailView) enterEdit() {
	detail, _, _ := v.app.Todos.DetailSnapshot()
	if detail == nil {
		return
	}
	t := detail.Todo
	v.titleEdit.Text = t.Title
	v.descEdit.Text = t.Description
	v.priorityEdit.Text = t.Priority
	v.tagsEdit.Text = strings.Join(t.Tags, ", ")
	if t.DueAt != nil {
		v.dueEdit.Text = *t.DueAt
	} else {
		v.dueEdit.Text = ""
	}
	v.titleEdit.Refresh()
	v.descEdit.Refresh()
	v.priorityEdit.Refresh()
	v.tagsEdit.Refresh()
	v.dueEdit.Refresh()

	v.editForm.Show()
}

// saveEdit PUTs the edited fields and refreshes.
func (v *DetailView) saveEdit() {
	detail, _, _ := v.app.Todos.DetailSnapshot()
	if detail == nil {
		return
	}
	idStr := store.IDString(detail.Todo.ID)
	body := map[string]any{
		"title":       v.titleEdit.Text,
		"description": v.descEdit.Text,
		"priority":    normalisePriority(v.priorityEdit.Text),
		"tags":        parseTags(v.tagsEdit.Text),
	}
	if due := strings.TrimSpace(v.dueEdit.Text); due != "" {
		body["due_at"] = due
	} else {
		body["due_at"] = nil
	}
	c := v.app.State.Client
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := c.UpdateTodo(ctx, idStr, body); err != nil {
			v.app.SetMessage(i18n.T("detail.saveFailed") + err.Error())
			return
		}
		fyne.Do(func() {
			v.app.refreshList()
			v.app.OpenDetail(detail.Todo.ID) // reload detail
			v.editForm.Hide()
		})
	}()
}

// sendComment posts a new comment and refreshes.
func (v *DetailView) sendComment() {
	detail, _, _ := v.app.Todos.DetailSnapshot()
	if detail == nil {
		return
	}
	content := strings.TrimSpace(v.commentEntry.Text)
	if content == "" {
		return
	}
	idStr := store.IDString(detail.Todo.ID)
	c := v.app.State.Client
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := c.CreateComment(ctx, idStr, content); err != nil {
			v.app.SetMessage(i18n.T("detail.commentFailed") + err.Error())
			return
		}
		fyne.Do(func() {
			v.commentEntry.Text = ""
			v.commentEntry.Refresh()
			v.app.OpenDetail(detail.Todo.ID)
		})
	}()
}

// deleteComment removes a comment and refreshes.
func (v *DetailView) deleteComment(commentID uint) {
	detail, _, _ := v.app.Todos.DetailSnapshot()
	if detail == nil {
		return
	}
	idStr := store.IDString(detail.Todo.ID)
	cStr := fmt.Sprintf("%d", commentID)
	c := v.app.State.Client
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := c.DeleteComment(ctx, idStr, cStr); err != nil {
			v.app.SetMessage(i18n.T("detail.delCommentFail") + err.Error())
			return
		}
		fyne.Do(func() {
			v.app.OpenDetail(detail.Todo.ID)
		})
	}()
}

// --- helpers --------------------------------------------------------------

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func formatTime(s string) string {
	if s == "" {
		return "-"
	}
	// Best-effort RFC3339 parse → local display.
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Local().Format("2006-01-02 15:04")
}

// formatDisplayCode renders a todo code with its category prefix (T-1/B-2/F-3),
// matching the web client's display convention. The backend stores bare
// numeric codes; the prefix is purely presentational.
func formatDisplayCode(category, code string) string {
	prefix := "?"
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "bug":
		prefix = "B"
	case "feature":
		prefix = "F"
	case "task":
		prefix = "T"
	}
	if strings.TrimSpace(code) == "" {
		return prefix
	}
	return prefix + "-" + code
}

func summaryToText(s client.TodoSummary) string {
	return fmt.Sprintf("%s  %s", formatDisplayCode(s.Category, s.Code), s.Title)
}

func summariesToText(ss []client.TodoSummary) string {
	if len(ss) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(ss))
	for _, s := range ss {
		parts = append(parts, summaryToText(s))
	}
	return strings.Join(parts, "\n")
}

func normalisePriority(p string) string {
	p = strings.ToLower(strings.TrimSpace(p))
	if p == "" {
		return "p2"
	}
	return p
}

func parseTags(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
