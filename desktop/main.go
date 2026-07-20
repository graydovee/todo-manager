// Command todo-desktop is the desktop companion client for the todo-manager
// backend, rewritten from Gio to Fyne v2.8. The window is borderless and
// side-panel based: left = todo list, right = detail / manage / create. Window
// mode (always-on-top / click-through lock) is driven through the platform
// package on Windows.
package main

import (
	"log"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/graydovee/todo-manager/desktop/internal/config"
	"github.com/graydovee/todo-manager/desktop/internal/i18n"
	"github.com/graydovee/todo-manager/desktop/internal/platform"
	"github.com/graydovee/todo-manager/desktop/internal/store"
	"github.com/graydovee/todo-manager/desktop/internal/theme"
	"github.com/graydovee/todo-manager/desktop/internal/ui"
)

func main() {
	// Resolve home dir for config + i18n.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("resolve user home directory: %v", err)
	}

	// Load config (or fall back to defaults).
	cfg, err := config.Load(homeDir)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Bootstrap app state + stores.
	state := store.NewAppState(cfg)
	todos := store.NewTodoStore()

	// Fyne app: borderless, custom minimalist theme.
	fyneApp := app.New()
	fyneApp.Settings().SetTheme(theme.Minimal)

	// Compute window size: bump width to accommodate the side panel.
	width := cfg.Window.Width
	height := cfg.Window.Height
	if width < 500 {
		width = 500
	}
	if height < 560 {
		height = 560
	}

	win := fyneApp.NewWindow(i18n.T("common.appName"))
	win.SetDecorated(false)
	win.Resize(fyne.NewSize(float32(width), float32(height)))

	// Build the central App controller (mounts UI, sets close intercept).
	a := ui.New(fyneApp, win, state, todos, homeDir)

	// Wire the system tray.
	tray := platform.NewTray(fyneApp, platform.TrayCallbacks{
		OnToggleLock: func() {
			a.SetLock(!state.Locked)
		},
		OnToggleTopMost: func() {
			a.SetTopMost(!state.TopMost)
		},
		OnQuit: func() {
			a.Quit()
		},
	})
	a.SetTray(tray)

	// Show the appropriate initial page (login or list).
	a.showPage()

	win.Show()
	a.Run()
	fyneApp.Run()

	// Persist config on exit.
	if err := config.Write(homeDir, cfg); err != nil {
		log.Printf("persist config: %v", err)
	}
}
