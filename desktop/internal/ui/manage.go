package ui

import (
	"context"
	"strconv"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/graydovee/todo-manager/desktop/internal/i18n"
	"github.com/graydovee/todo-manager/desktop/internal/store"
)

// ManageUI is the filter + create + logout screen.
type ManageUI struct {
	app *App

	backBtn    widget.Clickable
	logoutBtn  widget.Clickable
	applyBtn   widget.Clickable
	createBtn  widget.Clickable
	cancelBtn  widget.Clickable
	langEnBtn  widget.Clickable
	langZhBtn  widget.Clickable

	// Filters (text editors / simple multi-toggle chips).
	qEditor      widget.Editor
	codeEditor   widget.Editor
	statusToggles map[string]*widget.Clickable
	categoryToggles map[string]*widget.Clickable
	priorityToggles map[string]*widget.Clickable

	// New todo form.
	creating bool
	newTitle widget.Editor
	newDesc  widget.Editor
	newCat   widget.Editor
	newPri   widget.Editor
	newTags  widget.Editor
	newDue   widget.Editor

	// Dock settings editors.
	animEditor   widget.Editor
	hideEditor   widget.Editor

	// hideHeader, when true, suppresses the header row (back button + title +
	// logout). The side window draws its own top bar instead.
	hideHeader bool
	// onBack, if set, overrides the default back-button behaviour.
	onBack func()

	// scroll persists the scroll position of the body content across frames.
	scroll layout.List
}

var (
	statusOptions    = []string{"open", "in_progress", "completed", "duplicate"}
	categoryOptions  = []string{"bug", "feature", "task"}
	priorityOptions  = []string{"p0", "p1", "p2", "p3"}
)

func NewManageUI(a *App) *ManageUI {
	m := &ManageUI{app: a}
	m.qEditor.SingleLine = true
	m.codeEditor.SingleLine = true
	m.newTitle.SingleLine = true
	m.newCat.SingleLine = true
	m.newPri.SingleLine = true
	m.newTags.SingleLine = true
	m.newDue.SingleLine = true
	m.animEditor.SingleLine = true
	m.hideEditor.SingleLine = true

	m.statusToggles = make(map[string]*widget.Clickable, len(statusOptions))
	m.categoryToggles = make(map[string]*widget.Clickable, len(categoryOptions))
	m.priorityToggles = make(map[string]*widget.Clickable, len(priorityOptions))
	for _, s := range statusOptions {
		m.statusToggles[s] = new(widget.Clickable)
	}
	for _, c := range categoryOptions {
		m.categoryToggles[c] = new(widget.Clickable)
	}
	for _, p := range priorityOptions {
		m.priorityToggles[p] = new(widget.Clickable)
	}

	// Seed filter editors from config.
	f := a.State.Config.Filters
	m.qEditor.SetText(f.Query)
	m.codeEditor.SetText(f.Code)
	// Seed dock timing editors from config (show default when unset).
	animMs := a.State.Config.Dock.AnimMs
	if animMs <= 0 {
		animMs = 500
	}
	hideMs := a.State.Config.Dock.HideDelayMs
	if hideMs <= 0 {
		hideMs = 600
	}
	m.animEditor.SetText(strconv.Itoa(animMs))
	m.hideEditor.SetText(strconv.Itoa(hideMs))
	m.scroll.Axis = layout.Vertical
	return m
}

func (m *ManageUI) Layout(gtx layout.Context, w *app.Window) layout.Dimensions {
	m.handleClicks(gtx)

	body := m.filtersView
	if m.creating {
		body = m.createForm
	}

	children := make([]layout.FlexChild, 0, 3)
	if !m.hideHeader {
		children = append(children,
			layout.Rigid(m.header),
			layout.Rigid(separator),
		)
	}
	children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return m.scroll.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
				return body(gtx)
			})
		})
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (m *ManageUI) header(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return iconButton(gtx, m.app.Theme, &m.backBtn, IconBack, false) }),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					t := material.Body1(m.app.Theme, i18n.T("manage.title"))
					t.Font.Weight = font.SemiBold
					t.Color = textPrimary
					return t.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return smallButton(gtx, m.app.Theme, &m.logoutBtn, i18n.T("manage.logout")) }),
			)
		},
	)
}

// langRow renders two chip buttons (中文 / English); the active one is filled.
func (m *ManageUI) langRow(gtx layout.Context) layout.Dimensions {
	cur := i18n.Default.Lang()
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return chipButton(gtx, m.app.Theme, &m.langZhBtn, "中文", cur == i18n.Zh)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return chipButton(gtx, m.app.Theme, &m.langEnBtn, "English", cur == i18n.En)
		}),
	)
}

// switchLanguage changes the UI language, persists it, and repaints.
func (m *ManageUI) switchLanguage(l i18n.Lang) {
	i18n.Default.SetLang(l)
	m.app.State.Lock()
	m.app.State.Config.Language = string(l)
	m.app.State.Unlock()
	homePersist(m.app.State.Config)
	if m.app.Invalidate != nil {
		m.app.Invalidate()
	}
}

func (m *ManageUI) filtersView(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Language switcher.
		layout.Rigid(sectionLabel(m.app.Theme, i18n.T("manage.language"))),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(m.langRow),
		layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),

		// Dock timing settings.
		layout.Rigid(sectionLabel(m.app.Theme, i18n.T("manage.animDuration"))),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return labeledEditor(gtx, m.app.Theme, &m.animEditor, i18n.T("manage.animDuration"), "500")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(sectionLabel(m.app.Theme, i18n.T("manage.hideDelay"))),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return labeledEditor(gtx, m.app.Theme, &m.hideEditor, i18n.T("manage.hideDelay"), "600")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),

		layout.Rigid(sectionLabel(m.app.Theme, i18n.T("manage.search"))),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return labeledEditor(gtx, m.app.Theme, &m.qEditor, i18n.T("manage.titleSearch"), "")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return labeledEditor(gtx, m.app.Theme, &m.codeEditor, i18n.T("manage.codeExact"), "")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),

		layout.Rigid(sectionLabel(m.app.Theme, i18n.T("manage.status"))),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(m.toggleRow(statusOptions, m.statusToggles)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),

		layout.Rigid(sectionLabel(m.app.Theme, i18n.T("manage.category"))),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(m.toggleRow(categoryOptions, m.categoryToggles)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),

		layout.Rigid(sectionLabel(m.app.Theme, i18n.T("manage.priority"))),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(m.toggleRow(priorityOptions, m.priorityToggles)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(18)}.Layout),

		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return smallButton(gtx, m.app.Theme, &m.applyBtn, i18n.T("manage.apply"))
		}),
	)
}

// toggleRow renders a horizontal row of on/off chip buttons.
func (m *ManageUI) toggleRow(options []string, toggles map[string]*widget.Clickable) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		flex := layout.Flex{Axis: layout.Horizontal}
		children := make([]layout.FlexChild, 0, len(options))
		active := m.activeSet(toggles)
		for _, opt := range options {
			opt := opt
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return chipButton(gtx, m.app.Theme, toggles[opt], labelForOption(opt), active[opt])
			}))
			children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout))
		}
		return flex.Layout(gtx, children...)
	}
}

// activeSet reports which of the three filter slices contain each option, keyed
// by option name. The toggles argument is only used to pick which set to read.
func (m *ManageUI) activeSet(toggles map[string]*widget.Clickable) map[string]bool {
	out := map[string]bool{}
	m.app.State.Lock()
	defer m.app.State.Unlock()
	f := m.app.State.Config.Filters
	var values []string
	switch {
	case toggles["open"] != nil || toggles["completed"] != nil:
		values = f.Status
	case toggles["bug"] != nil || toggles["task"] != nil:
		values = f.Category
	case toggles["p0"] != nil || toggles["p3"] != nil:
		values = f.Priority
	}
	for _, v := range values {
		out[v] = true
	}
	return out
}

func (m *ManageUI) createForm(gtx layout.Context) layout.Dimensions {
	if m.newCat.Text() == "" {
		m.newCat.SetText("task")
		m.newPri.SetText("p2")
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(sectionLabel(m.app.Theme, i18n.T("manage.newTodo"))),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return labeledEditor(gtx, m.app.Theme, &m.newTitle, i18n.T("manage.titleLabel"), "") }),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return labeledEditor(gtx, m.app.Theme, &m.newDesc, i18n.T("manage.descLabel"), "") }),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return labeledEditor(gtx, m.app.Theme, &m.newCat, i18n.T("manage.catLabel"), "") }),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return labeledEditor(gtx, m.app.Theme, &m.newPri, i18n.T("manage.priLabel"), "") }),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return labeledEditor(gtx, m.app.Theme, &m.newTags, i18n.T("manage.tagsLabel"), "") }),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return labeledEditor(gtx, m.app.Theme, &m.newDue, i18n.T("manage.dueLabel"), "") }),
		layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return smallButton(gtx, m.app.Theme, &m.createBtn, i18n.T("manage.create")) }),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return smallButton(gtx, m.app.Theme, &m.cancelBtn, i18n.T("common.cancel")) }),
			)
		}),
	)
}

// --- click handling --------------------------------------------------------

func (m *ManageUI) handleClicks(gtx layout.Context) {
	for m.backBtn.Clicked(gtx) {
		if m.onBack != nil {
			m.onBack()
		} else {
			m.app.nav().goTo(store.PageList)
		}
	}
	for m.logoutBtn.Clicked(gtx) {
		m.doLogout()
	}
	for m.langEnBtn.Clicked(gtx) {
		m.switchLanguage(i18n.En)
	}
	for m.langZhBtn.Clicked(gtx) {
		m.switchLanguage(i18n.Zh)
	}
	for m.applyBtn.Clicked(gtx) {
		m.applyFilters()
	}
	for m.cancelBtn.Clicked(gtx) {
		m.creating = false
		m.resetNewForm()
	}
	for m.createBtn.Clicked(gtx) {
		m.createTodo()
	}
	// Chip toggles flip membership immediately; apply on Apply.
	for opt, c := range m.statusToggles {
		for c.Clicked(gtx) {
			m.toggleFilter("status", opt)
		}
	}
	for opt, c := range m.categoryToggles {
		for c.Clicked(gtx) {
			m.toggleFilter("category", opt)
		}
	}
	for opt, c := range m.priorityToggles {
		for c.Clicked(gtx) {
			m.toggleFilter("priority", opt)
		}
	}
}

// toggleFilter adds/removes a value from the named filter slice.
func (m *ManageUI) toggleFilter(kind, value string) {
	m.app.State.Lock()
	defer m.app.State.Unlock()
	f := &m.app.State.Config.Filters
	var slice *[]string
	switch kind {
	case "status":
		slice = &f.Status
	case "category":
		slice = &f.Category
	case "priority":
		slice = &f.Priority
	default:
		return
	}
	for i, v := range *slice {
		if v == value {
			*slice = append((*slice)[:i], (*slice)[i+1:]...)
			return
		}
	}
	*slice = append(*slice, value)
}

func (m *ManageUI) applyFilters() {
	animMs, _ := strconv.Atoi(strings.TrimSpace(m.animEditor.Text()))
	hideMs, _ := strconv.Atoi(strings.TrimSpace(m.hideEditor.Text()))
	m.app.State.Lock()
	m.app.State.Config.Filters.Query = strings.TrimSpace(m.qEditor.Text())
	m.app.State.Config.Filters.Code = strings.TrimSpace(m.codeEditor.Text())
	m.app.State.Config.Dock.AnimMs = animMs
	m.app.State.Config.Dock.HideDelayMs = hideMs
	m.app.State.Unlock()
	homePersist(m.app.State.Config)
	m.app.List.RequestRefresh()
	if m.onBack != nil {
		m.onBack()
	} else {
		m.app.nav().goTo(store.PageList)
	}
}

func (m *ManageUI) doLogout() {
	m.app.State.Lock()
	m.app.State.Config.APIKey = ""
	m.app.State.Client = nil
	m.app.State.Page = store.PageLogin
	m.app.State.Unlock()
	homePersist(m.app.State.Config)
	if m.app.Invalidate != nil {
		m.app.Invalidate()
	}
}

func (m *ManageUI) resetNewForm() {
	m.newTitle.SetText("")
	m.newDesc.SetText("")
	m.newCat.SetText("")
	m.newPri.SetText("")
	m.newTags.SetText("")
	m.newDue.SetText("")
}

func (m *ManageUI) createTodo() {
	cl := m.app.Client()
	if cl == nil {
		return
	}
	title := strings.TrimSpace(m.newTitle.Text())
	if title == "" {
		m.app.State.SetMessage(i18n.T("manage.titleRequired"))
		return
	}
	cat := strings.ToLower(strings.TrimSpace(m.newCat.Text()))
	switch cat {
	case "bug", "feature", "task":
	default:
		m.app.State.SetMessage(i18n.T("manage.catInvalid"))
		return
	}
	body := map[string]any{
		"title":       title,
		"description": m.newDesc.Text(),
		"category":    cat,
		"priority":    normalizePriority(m.newPri.Text()),
		"tags":        parseTags(m.newTags.Text()),
	}
	if due := strings.TrimSpace(m.newDue.Text()); due != "" {
		body["due_at"] = due
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := cl.CreateTodo(ctx, body); err != nil {
			m.app.State.SetMessage(i18n.T("manage.createFailed") + err.Error())
			return
		}
		m.creating = false
		m.resetNewForm()
		m.app.State.SetMessage(i18n.T("manage.created"))
		m.app.List.RequestRefresh()
		if m.onBack != nil {
			m.onBack()
		} else {
			m.app.nav().goTo(store.PageList)
		}
	}()
}

// --- helpers ---------------------------------------------------------------

func labelForOption(opt string) string {
	switch opt {
	case "open":
		return i18n.T("todo.open")
	case "in_progress":
		return i18n.T("todo.inProgress")
	case "completed":
		return i18n.T("todo.completed")
	case "duplicate":
		return i18n.T("todo.duplicate")
	}
	return opt
}

func chipButton(gtx layout.Context, th *material.Theme, c *widget.Clickable, label string, active bool) layout.Dimensions {
	btn := material.Button(th, c, label)
	styleChipButton(&btn, active)
	return uniformButton(gtx, &btn)
}

// ensure fmt stays used if labelForOption ever returns formatted strings.
