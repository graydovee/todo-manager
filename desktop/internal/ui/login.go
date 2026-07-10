package ui

import (
	"context"
	"image"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/graydovee/todo-manager/desktop/internal/client"
	"github.com/graydovee/todo-manager/desktop/internal/config"
	"github.com/graydovee/todo-manager/desktop/internal/i18n"
	"github.com/graydovee/todo-manager/desktop/internal/store"
)

// LoginUI is the connection screen: backend URL + API key + test.
type LoginUI struct {
	app     *App
	baseURL widget.Editor
	apiKey  widget.Editor
	testBtn widget.Clickable

	testing bool
	result  string // status message
}

func NewLoginUI(a *App) *LoginUI {
	u := &LoginUI{app: a}
	u.baseURL.SingleLine = true
	u.baseURL.Submit = true
	u.apiKey.SingleLine = true
	u.apiKey.Submit = true

	cfg := a.State.Config
	u.baseURL.SetText(cfg.BaseURL)
	return u
}

func (u *LoginUI) Layout(gtx layout.Context, w *app.Window) layout.Dimensions {
	// Process clicks every frame (Gio immediate-mode pattern).
	for u.testBtn.Clicked(gtx) {
		u.tryConnect()
	}
	u.baseURL.Update(gtx)
	u.apiKey.Update(gtx)

	// Pad the whole window content.
	return layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// Title.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				title := material.H5(u.app.Theme, i18n.T("login.title"))
				title.Color = textPrimary
				return title.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				sub := material.Body2(u.app.Theme, i18n.T("login.subtitle"))
				sub.Color = textSecondary
				return sub.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(24)}.Layout),

			// Base URL.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return labeledEditor(gtx, u.app.Theme, &u.baseURL, i18n.T("login.baseURL"), config.DefaultBaseURL)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),

			// API key.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return labeledEditor(gtx, u.app.Theme, &u.apiKey, i18n.T("login.apiKey"), "")
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(20)}.Layout),

			// Connect button.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btn := material.Button(u.app.Theme, &u.testBtn, i18n.T("login.connect"))
				styleButton(&btn)
				if u.testing {
					btn.Text = i18n.T("login.connecting")
					btn.Background = textDisabled
				}
				return btn.Layout(gtx)
			}),

			// Status / hint.
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				msg := u.result
				if u.testing {
					msg = ""
				}
				if msg == "" {
					msg = i18n.T("login.tip")
				}
				lbl := material.Body2(u.app.Theme, msg)
				lbl.Color = textMuted
				return lbl.Layout(gtx)
			}),
		)
	})
}

// tryConnect runs the connectivity check in a goroutine.
func (u *LoginUI) tryConnect() {
	if u.testing {
		return
	}
	base := u.baseURL.Text()
	key := u.apiKey.Text()
	if base == "" || key == "" {
		u.result = i18n.T("login.emptyFields")
		if u.app.Invalidate != nil {
			u.app.Invalidate()
		}
		return
	}

	u.testing = true
	u.result = ""
	if u.app.Invalidate != nil {
		u.app.Invalidate()
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		c := client.New(base, key)
		// First check reachability, then auth.
		if err := c.Ping(ctx); err != nil {
			u.finish(false, i18n.T("login.unreachable")+err.Error())
			return
		}
		if err := c.CheckAuth(ctx); err != nil {
			u.finish(false, i18n.T("login.authFailed")+err.Error())
			return
		}

		// Persist config + enter the app.
		u.app.State.Lock()
		cfg := u.app.State.Config
		cfg.BaseURL = client.NormalizeBaseURL(base)
		cfg.APIKey = key
		u.app.State.Client = client.New(cfg.BaseURL, key)
		u.app.State.Page = store.PageList
		u.app.State.Unlock()

		homePersist(cfg)
		u.finish(true, "")
	}()
}

func (u *LoginUI) finish(ok bool, msg string) {
	u.testing = false
	u.result = msg
	if u.app.Invalidate != nil {
		u.app.Invalidate()
	}
	// On success, trigger an initial list load.
	if ok {
		u.app.List.RequestRefresh()
	}
}

func labeledEditor(gtx layout.Context, th *material.Theme, ed *widget.Editor, label, hint string) layout.Dimensions {
	ed.Update(gtx)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, label)
			lbl.Color = textSecondary
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// Lay out the editor, then draw an underline beneath it. Focused
			// editors get a darker line.
			de := material.Editor(th, ed, hint)
			de.Color = textPrimary
			de.HintColor = textMuted
			dims := de.Layout(gtx)
			// Underline: shift down by the editor height and fill a 1px line.
			lineColor := border
			if gtx.Source.Focused(ed) {
				lineColor = textSecondary
			}
			defer op.Offset(image.Pt(0, dims.Size.Y)).Push(gtx.Ops).Pop()
			paint.FillShape(gtx.Ops, lineColor, clip.Rect{Max: image.Pt(dims.Size.X, 1)}.Op())
			return layout.Dimensions{Size: image.Pt(dims.Size.X, dims.Size.Y + 1)}
		}),
	)
}
