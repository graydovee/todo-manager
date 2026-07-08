// Command todo-desktop is a small always-on-top todo widget built with Gio. It
// authenticates to the todo-manager backend with a Bearer API key.
package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/graydovee/todo-manager/desktop/internal/config"
	"github.com/graydovee/todo-manager/desktop/internal/i18n"
	"github.com/graydovee/todo-manager/desktop/internal/platform"
	"github.com/graydovee/todo-manager/desktop/internal/store"
	"github.com/graydovee/todo-manager/desktop/internal/ui"
)

// logFile is where diagnostic output (panics, lifecycle markers) is written.
// Set in main() so recover/logf can use it.
var logFile *os.File

func logf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stderr, msg)
	if logFile != nil {
		fmt.Fprintln(logFile, msg)
	}
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "resolve home directory:", err)
		os.Exit(1)
	}

	// Open a diagnostic log next to the config so crashes are visible even with
	// -H windowsgui (no console). Ensure the directory exists first.
	logDir := home + "/.todo-manager"
	_ = os.MkdirAll(logDir, 0o755)
	if f, ferr := os.Create(logDir + "/desktop.log"); ferr == nil {
		logFile = f
		defer logFile.Close()
		platform.SetLogFile(f)
	}
	logf("todo-desktop starting; home=%q", home)

	cfg, err := config.Load(home)
	if err != nil {
		logf("load config: %v", err)
		os.Exit(1)
	}

	ui.SetHomeDir(home)
	state := store.NewAppState(cfg)
	todos := store.NewTodoStore()
	theme := ui.NewTheme()
	logf("config loaded; baseURL=%q hasKey=%v page=%d", cfg.BaseURL, cfg.APIKey != "", state.Page)

	// Per Gio's documented pattern on Windows/Unix, the window event loop runs
	// in a goroutine and app.Main() owns the main thread (it blocks). Running
	// the loop on the main goroutine without Main() can deadlock or hang.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logf("PANIC in runWindow: %v\n%s", r, debug.Stack())
				os.Exit(1)
			}
		}()
		runWindow(state, todos, theme, cfg, home)
	}()

	app.Main()
}

// runWindow drives the Gio window event loop. It runs on a goroutine so the
// main goroutine can call app.Main().
func runWindow(state *store.AppState, todos *store.TodoStore, theme *material.Theme, cfg *config.Config, home string) {
	w := &app.Window{}
	w.Option(app.Size(unit.Dp(dim(cfg.Window.Width, 360)), unit.Dp(dim(cfg.Window.Height, 560))))
	w.Option(app.Decorated(false))
	w.Option(app.Title(i18n.T("common.appName")))

	gui := ui.NewApp(theme, state, todos)
	gui.Invalidate = func() { w.Invalidate() }

	// Tray runs on a dedicated goroutine with its own hidden-window message loop,
	// entirely separate from Gio's event loop. Menu clicks arrive on trayCmds and
	// are drained between Gio events. invalidate wakes the blocking w.Event() so
	// the loop can process tray commands promptly even without window input.
	trayCmds := make(chan platform.TrayCmd, 8)
	go platform.RunTray(cfg.Window.TopMost, trayCmds, func() { w.Invalidate() })

	logf("runWindow: entering event loop")

	var ops op.Ops
	var frameCount int
	for {
		// Drain pending tray actions before waiting for the next window event.
		if handleTray(gui, w, trayCmds, home) {
			exitApp(home, gui.State.Config)
		}

		e := w.Event()
		switch e := e.(type) {
		case app.ViewEvent:
			logf("event: ViewEvent valid=%v", e.Valid())
			h := platform.ExtractHandle(e)
			if h != 0 && gui.Platform == nil {
				gui.Platform = platform.NewController(platform.Handle(h))
				logf("platform controller installed hwnd=%v", h)
				// Only touch native window flags when the persisted mode actually
				// requires it. Calling SetWindowLongPtr / SetWindowPos on a freshly
				// created window for a no-op (locked=false, topmost=false) can
				// synchronously deliver style-change messages that deadlock against
				// Gio's window thread. Skip entirely when the defaults apply.
				if state.Locked {
					gui.SetLock(true)
				} else if state.TopMost {
					gui.SetTopMost(true)
				} else {
					logf("no persisted lock/topmost; skipping native flags")
				}
				// Keep the window out of the taskbar/Alt+Tab; the tray icon is the
				// only taskbar presence.
				gui.Platform.HideFromTaskbar()
			}
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			gui.Layout(gtx, w)
			e.Frame(gtx.Ops)
			frameCount++
			if frameCount <= 3 {
				logf("event: FrameEvent #%d size=%v", frameCount, e.Size)
			}
		case app.DestroyEvent:
			logf("event: DestroyEvent err=%v", e.Err)
			exitApp(home, gui.State.Config)
		}
	}
}

// exitApp persists the config and terminates the process. app.Main() blocks
// forever on Windows, so an explicit exit is required on any shutdown path.
func exitApp(home string, cfg *config.Config) {
	_ = config.Write(home, cfg)
	os.Exit(0)
}

// handleTray drains queued tray actions. It returns true if the app should
// quit. It runs on the window goroutine, between Gio events.
func handleTray(gui *ui.App, w *app.Window, cmds <-chan platform.TrayCmd, home string) (quit bool) {
	for {
		select {
		case c, ok := <-cmds:
			if !ok {
				// Channel closed by the tray bridge on quit.
				_ = config.Write(home, gui.State.Config)
				return true
			}
			switch c {
			case platform.TrayUnlock:
				gui.SetLock(false)
				platform.SetTrayTopMost(gui.IsTopMost())
				w.Invalidate()
			case platform.TrayToggleTopMost:
				gui.SetTopMost(!gui.IsTopMost())
				platform.SetTrayTopMost(gui.IsTopMost())
				w.Invalidate()
			case platform.TrayQuit:
				_ = config.Write(home, gui.State.Config)
				return true
			}
		default:
			return false
		}
	}
}

// dim clamps a configured dimension to a sensible minimum.
func dim(v, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
}
