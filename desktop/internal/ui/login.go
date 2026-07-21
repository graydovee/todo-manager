package ui

import (
	"context"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/graydovee/todo-manager/desktop/internal/client"
	"github.com/graydovee/todo-manager/desktop/internal/i18n"
	"github.com/graydovee/todo-manager/desktop/internal/store"
)

// LoginView renders the connect screen: backend URL + API key entries plus a
// Connect button. Connect performs a two-step check (Ping then CheckAuth).
type LoginView struct {
	app *App

	root       *fyne.Container
	baseURL    *widget.Entry
	apiKey     *widget.Entry
	connectBtn *widget.Button
	status     *widget.Label
}

func newLoginView(app *App) *LoginView {
	v := &LoginView{app: app}

	v.baseURL = widget.NewEntry()
	v.baseURL.SetPlaceHolder(i18n.T("login.baseURL"))
	v.baseURL.Text = app.State.Config.BaseURL

	v.apiKey = widget.NewPasswordEntry()
	v.apiKey.SetPlaceHolder(i18n.T("login.apiKey"))
	v.apiKey.Text = app.State.Config.APIKey

	v.status = widget.NewLabel("")
	v.status.Wrapping = fyne.TextWrapWord

	v.connectBtn = widget.NewButton(i18n.T("login.connect"), v.connect)
	v.connectBtn.Importance = widget.HighImportance

	form := container.NewVBox(
		widget.NewLabelWithStyle(i18n.T("login.title"), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle(i18n.T("login.subtitle"), fyne.TextAlignCenter, fyne.TextStyle{}),
		widget.NewLabel(i18n.T("login.baseURL")),
		v.baseURL,
		widget.NewLabel(i18n.T("login.apiKey")),
		v.apiKey,
		v.connectBtn,
		widget.NewLabel(i18n.T("login.tip")),
		v.status,
	)
	v.root = container.NewCenter(container.NewVBox(form))
	return v
}

// Build returns the root canvas object for the login page.
func (v *LoginView) Build() fyne.CanvasObject {
	return v.root
}

// connect performs the two-step connectivity check then switches to the list
// page. All network work happens in a goroutine; UI updates are deferred to
// fyne.Do.
func (v *LoginView) connect() {
	base := v.baseURL.Text
	key := v.apiKey.Text
	if base == "" || key == "" {
		v.status.SetText(i18n.T("login.emptyFields"))
		return
	}

	v.connectBtn.Disable()
	v.connectBtn.SetText(i18n.T("login.connecting"))
	v.status.SetText("")

	go func() {
		c := client.New(base, key)

		// Step 1: ping (server reachable).
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer pingCancel()
		if err := c.Ping(pingCtx); err != nil {
			fyne.Do(func() {
				v.status.SetText(i18n.T("login.unreachable") + err.Error())
				v.connectBtn.Enable()
				v.connectBtn.SetText(i18n.T("login.connect"))
			})
			return
		}

		// Step 2: auth check (key valid).
		authCtx, authCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer authCancel()
		if err := c.CheckAuth(authCtx); err != nil {
			fyne.Do(func() {
				v.status.SetText(i18n.T("login.authFailed") + err.Error())
				v.connectBtn.Enable()
				v.connectBtn.SetText(i18n.T("login.connect"))
			})
			return
		}

		// Success: commit state + switch page on the UI goroutine.
		fyne.Do(func() {
			v.app.State.Login(base, key)
			v.connectBtn.Enable()
			v.connectBtn.SetText(i18n.T("login.connect"))
			v.app.State.Page = store.PageList
			v.app.ShowPage()
		})
	}()
}
