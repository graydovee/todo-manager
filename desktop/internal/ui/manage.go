package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/graydovee/todo-manager/desktop/internal/i18n"
)

// ManageView renders the management side panel: language switch, dock settings,
// filters (status/category/priority chips), title/code search, apply + logout.
// It also renders the create-todo form via BuildCreate() when the user opens
// the create side panel.
type ManageView struct {
	app *App

	root       fyne.CanvasObject
	createRoot fyne.CanvasObject

	// Filters.
	statusChips   *chipGroup
	categoryChips *chipGroup
	priorityChips *chipGroup
	queryEntry    *widget.Entry
	codeEntry     *widget.Entry
	applyBtn      *widget.Button

	// Dock + language.
	langSelect   *widget.Select
	animEntry    *widget.Entry
	hideDelayEnt *widget.Entry

	// Logout.
	logoutBtn *widget.Button

	// Create form.
	titleEntry *widget.Entry
	descEntry  *widget.Entry
	catEntry   *widget.Entry
	priEntry   *widget.Entry
	tagsEntry  *widget.Entry
	dueEntry   *widget.Entry
	createBtn  *widget.Button
	createMsg  *widget.Label
}

func newManageView(app *App) *ManageView {
	v := &ManageView{app: app}

	// Filter chips.
	v.statusChips = newChipGroup([]string{"open", "in_progress", "completed", "duplicate"},
		labelsForStatuses, app.State.Config.Filters.Status)
	v.categoryChips = newChipGroup([]string{"bug", "feature", "task"},
		labelsForCategories, app.State.Config.Filters.Category)
	v.priorityChips = newChipGroup([]string{"p0", "p1", "p2", "p3"},
		identity, app.State.Config.Filters.Priority)

	v.queryEntry = widget.NewEntry()
	v.queryEntry.SetPlaceHolder(i18n.T("manage.titleSearch"))
	v.queryEntry.Text = app.State.Config.Filters.Query

	v.codeEntry = widget.NewEntry()
	v.codeEntry.SetPlaceHolder(i18n.T("manage.codeExact"))
	v.codeEntry.Text = app.State.Config.Filters.Code

	v.applyBtn = widget.NewButton(i18n.T("manage.apply"), v.applyFilters)

	// Language + dock settings.
	v.langSelect = widget.NewSelect([]string{"English", "简体中文"}, v.onLanguageChange)
	cur := "English"
	if app.State.I18n.Lang() == "zh" {
		cur = "简体中文"
	}
	v.langSelect.SetSelected(cur)

	v.animEntry = widget.NewEntry()
	v.animEntry.Text = fmt.Sprintf("%d", app.State.Config.Dock.AnimMs)
	v.hideDelayEnt = widget.NewEntry()
	v.hideDelayEnt.Text = fmt.Sprintf("%d", app.State.Config.Dock.HideDelayMs)

	// Logout.
	v.logoutBtn = widget.NewButton(i18n.T("manage.logout"), v.logout)

	// Build the manage layout.
	v.root = container.NewVScroll(container.NewPadded(container.NewVBox(
		widget.NewLabelWithStyle(i18n.T("manage.language"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		v.langSelect,
		widget.NewLabel(i18n.T("manage.animDuration")),
		v.animEntry,
		widget.NewLabel(i18n.T("manage.hideDelay")),
		v.hideDelayEnt,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(i18n.T("manage.status"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		v.statusChips.Build(),
		widget.NewLabelWithStyle(i18n.T("manage.category"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		v.categoryChips.Build(),
		widget.NewLabelWithStyle(i18n.T("manage.priority"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		v.priorityChips.Build(),
		widget.NewLabel(i18n.T("manage.titleSearch")),
		v.queryEntry,
		widget.NewLabel(i18n.T("manage.codeExact")),
		v.codeEntry,
		v.applyBtn,
		widget.NewSeparator(),
		v.logoutBtn,
	)))

	// Build the create form.
	v.titleEntry = widget.NewEntry()
	v.titleEntry.SetPlaceHolder(i18n.T("manage.titleLabel"))
	v.descEntry = widget.NewMultiLineEntry()
	v.descEntry.SetPlaceHolder(i18n.T("manage.descLabel"))
	v.descEntry.SetMinRowsVisible(3)
	v.catEntry = widget.NewEntry()
	v.catEntry.SetPlaceHolder(i18n.T("manage.catLabel"))
	v.catEntry.Text = "task"
	v.priEntry = widget.NewEntry()
	v.priEntry.SetPlaceHolder(i18n.T("manage.priLabel"))
	v.priEntry.Text = "p2"
	v.tagsEntry = widget.NewEntry()
	v.tagsEntry.SetPlaceHolder(i18n.T("manage.tagsLabel"))
	v.dueEntry = widget.NewEntry()
	v.dueEntry.SetPlaceHolder(i18n.T("manage.dueLabel"))
	v.createBtn = widget.NewButton(i18n.T("manage.create"), v.createTodo)
	v.createMsg = widget.NewLabel("")

	v.createRoot = container.NewVScroll(container.NewPadded(container.NewVBox(
		widget.NewLabel(i18n.T("manage.titleLabel")),
		v.titleEntry,
		widget.NewLabel(i18n.T("manage.descLabel")),
		v.descEntry,
		widget.NewLabel(i18n.T("manage.catLabel")),
		v.catEntry,
		widget.NewLabel(i18n.T("manage.priLabel")),
		v.priEntry,
		widget.NewLabel(i18n.T("manage.tagsLabel")),
		v.tagsEntry,
		widget.NewLabel(i18n.T("manage.dueLabel")),
		v.dueEntry,
		v.createBtn,
		v.createMsg,
	)))

	return v
}

// Build returns the manage panel root.
func (v *ManageView) Build() fyne.CanvasObject {
	return v.root
}

// BuildCreate returns the create-todo panel root.
func (v *ManageView) BuildCreate() fyne.CanvasObject {
	return v.createRoot
}

// Refresh re-syncs the chip selection from the config (used when first shown).
func (v *ManageView) Refresh() {
	cfg := v.app.State.Config
	v.statusChips.SetSelected(cfg.Filters.Status)
	v.categoryChips.SetSelected(cfg.Filters.Category)
	v.priorityChips.SetSelected(cfg.Filters.Priority)
}

// ShowCreate resets the create form for a fresh entry.
func (v *ManageView) ShowCreate() {
	v.titleEntry.Text = ""
	v.descEntry.Text = ""
	v.catEntry.Text = "task"
	v.priEntry.Text = "p2"
	v.tagsEntry.Text = ""
	v.dueEntry.Text = ""
	v.createMsg.SetText("")
	for _, e := range []*widget.Entry{v.titleEntry, v.descEntry, v.catEntry, v.priEntry, v.tagsEntry, v.dueEntry} {
		e.Refresh()
	}
}

// applyFilters writes the chip/entry state into config and refreshes the list.
func (v *ManageView) applyFilters() {
	v.app.State.Lock()
	cfg := v.app.State.Config
	cfg.Filters.Status = v.statusChips.Selected()
	cfg.Filters.Category = v.categoryChips.Selected()
	cfg.Filters.Priority = v.priorityChips.Selected()
	cfg.Filters.Query = strings.TrimSpace(v.queryEntry.Text)
	cfg.Filters.Code = strings.TrimSpace(v.codeEntry.Text)
	if am := atoiOr(v.animEntry.Text, 0); am > 0 {
		cfg.Dock.AnimMs = am
	}
	if hd := atoiOr(v.hideDelayEnt.Text, 0); hd > 0 {
		cfg.Dock.HideDelayMs = hd
	}
	v.app.State.Unlock()
	v.app.refreshList()
}

// onLanguageChange switches the i18n language and re-renders labels.
func (v *ManageView) onLanguageChange(lang string) {
	if lang == "简体中文" {
		v.app.State.I18n.SetLang("zh")
		v.app.State.Config.Language = "zh"
	} else {
		v.app.State.I18n.SetLang("en")
		v.app.State.Config.Language = "en"
	}
	v.app.syncTrayLabels()
}

// logout clears credentials and returns to the login page.
func (v *ManageView) logout() {
	v.app.State.Logout()
	v.app.showPage()
}

// createTodo POSTs a new todo and, on success, refreshes the list and opens the
// created todo's detail.
func (v *ManageView) createTodo() {
	title := strings.TrimSpace(v.titleEntry.Text)
	if title == "" {
		v.createMsg.SetText(i18n.T("manage.titleRequired"))
		return
	}
	category := strings.ToLower(strings.TrimSpace(v.catEntry.Text))
	if category == "" {
		category = "task"
	}
	switch category {
	case "bug", "feature", "task":
	default:
		v.createMsg.SetText(i18n.T("manage.catInvalid"))
		return
	}
	body := map[string]any{
		"title":    title,
		"category": category,
	}
	if d := strings.TrimSpace(v.descEntry.Text); d != "" {
		body["description"] = d
	}
	body["priority"] = normalisePriority(v.priEntry.Text)
	if tags := parseTags(v.tagsEntry.Text); len(tags) > 0 {
		body["tags"] = tags
	}
	if due := strings.TrimSpace(v.dueEntry.Text); due != "" {
		body["due_at"] = due
	}
	c := v.app.State.Client
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		t, err := c.CreateTodo(ctx, body)
		if err != nil {
			fyne.Do(func() { v.createMsg.SetText(i18n.T("manage.createFailed") + err.Error()) })
			return
		}
		fyne.Do(func() {
			v.createMsg.SetText(i18n.T("manage.created"))
			v.app.refreshList()
			v.app.OpenDetail(t.ID)
		})
	}()
}

// --- chipGroup ------------------------------------------------------------

// chipGroup is a row of toggleable chips (checkable buttons). It tracks which
// values are currently selected.
type chipGroup struct {
	values []string
	labels func(string) string
	btns   []*widget.Button
	sel    map[string]bool
}

func newChipGroup(values []string, labelFn func(string) string, initial []string) *chipGroup {
	g := &chipGroup{
		values: values,
		labels: labelFn,
		sel:    map[string]bool{},
	}
	for _, v := range initial {
		g.sel[v] = true
	}
	for _, v := range values {
		v := v
		b := widget.NewButton(labelFn(v), nil)
		b.Importance = widget.LowImportance
		b.OnTapped = func() {
			g.sel[v] = !g.sel[v]
			g.refreshButtons()
		}
		g.btns = append(g.btns, b)
	}
	g.refreshButtons()
	return g
}

// Build returns the container with all chip buttons.
func (g *chipGroup) Build() *fyne.Container {
	objs := make([]fyne.CanvasObject, 0, len(g.btns))
	for _, b := range g.btns {
		objs = append(objs, b)
	}
	return container.NewHBox(objs...)
}

// Selected returns the list of currently-selected values.
func (g *chipGroup) Selected() []string {
	out := make([]string, 0, len(g.sel))
	for _, v := range g.values {
		if g.sel[v] {
			out = append(out, v)
		}
	}
	return out
}

// SetSelected sets the selection from a list of values.
func (g *chipGroup) SetSelected(vals []string) {
	g.sel = map[string]bool{}
	for _, v := range vals {
		g.sel[v] = true
	}
	g.refreshButtons()
}

func (g *chipGroup) refreshButtons() {
	for _, b := range g.btns {
		// Use importance as a visual toggle proxy.
		b.Importance = widget.LowImportance
	}
	for i, v := range g.values {
		if g.sel[v] {
			g.btns[i].Importance = widget.HighImportance
		}
		g.btns[i].Refresh()
	}
}

// --- label helpers --------------------------------------------------------

func labelsForStatuses(s string) string { return statusLabel(s) }

func labelsForCategories(s string) string { return s }

func identity(s string) string { return s }

func atoiOr(s string, def int) int {
	s = strings.TrimSpace(s)
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	if s == "" {
		return def
	}
	return n
}
