// Command todo-desktop is a small always-on-top todo widget built with Gio. It
// authenticates to the todo-manager backend with a Bearer API key.
package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"time"

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

	// Dock poll goroutine: when the window is docked to an edge and unlocked,
	// auto-hide it (slide off-screen) and slide it back out when the cursor
	// approaches the edge.
	go dockPollLoop(gui, w)

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
				gui.OwnerHandle = platform.Handle(h)
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

// dockPollLoop watches cursor proximity to auto-hide/show a docked window.
// It runs on its own goroutine and polls every ~150ms. The window auto-hides
// (slides off-screen) only when: docked, unlocked, not dragging, and on the list
// page. When hidden, the cursor approaching the docked edge slides it back out.
func dockPollLoop(gui *ui.App, w *app.Window) {
	const (
		animInterval = 16  // ms between animation ticks (~60fps); only affects smoothness
		nearEdge     = 6   // px from edge to trigger slide-out
		gracePeriod  = 1000 // ms after slide-out before hiding is allowed again
	)
	var (
		lastLeftTime int64
		lastShownMs  int64 // when the window last finished sliding out
	)
	for {
		time.Sleep(animInterval * time.Millisecond)

		// Read tunable timing from config (0 = built-in default).
		animDuration := int64(gui.DockAnimMs())
		hideDelay := int64(gui.DockHideDelayMs())

		ctrl := gui.Platform
		if ctrl == nil {
			continue
		}

		dock := gui.DockSnapshot()

		// Animate an in-progress slide, driven by wall-clock time so the
		// duration is exact regardless of sleep/tick jitter.
		if dock.Animating {
			elapsed := time.Now().UnixMilli() - dock.AnimStartMs
			if elapsed >= animDuration {
				ctrl.MoveWindowSync(dock.AnimTargetX, dock.AnimTargetY, 0, 0)
				gui.StopAnim()
				// Record when the window settled so we can apply a grace period
				// before hiding again (prevents show/hide oscillation at edges).
				lastShownMs = time.Now().UnixMilli()
				continue
			}
			t := float32(elapsed) / float32(animDuration)
			// Ease-out for a natural deceleration feel.
			t = 1 - (1-t)*(1-t)
			nx := lerp(dock.AnimStartX, dock.AnimTargetX, t)
			ny := lerp(dock.AnimStartY, dock.AnimTargetY, t)
			ctrl.MoveWindowSync(nx, ny, 0, 0)
			continue
		}

		// Idle logic: auto-hide only when docked, top-most, unlocked, not
		// dragging, and on the list page.
		if !dock.Docked || dock.Dragging || gui.IsLocked() || !gui.IsTopMost() {
			lastLeftTime = 0
			continue
		}
		if gui.CurrentPage() != store.PageList {
			lastLeftTime = 0
			continue
		}

		cx, cy := ctrl.CursorPos()
		wx, wy, ww, wh := ctrl.WindowGeometry()
		workX, workY, workW, workH := ctrl.WorkArea()
		now := time.Now().UnixMilli()

		if dock.Hidden {
			// Hidden: slide out when cursor nears the docked edge.
			if cursorNearEdge(cx, cy, dock.Edge, dock.DockX, dock.DockY, ww, wh, nearEdge, workX, workY, workW, workH) {
				gui.SetDockHidden(false, dock.HideX, dock.HideY, now) // animate from hide→dock
			}
		} else {
			// Shown: hide when cursor leaves the window area (after a delay).
			// Skip during the grace period right after sliding out.
			if now-lastShownMs < gracePeriod {
				lastLeftTime = 0
			} else if cursorInWindow(cx, cy, wx, wy, ww, wh) {
				lastLeftTime = 0
			} else {
				if lastLeftTime == 0 {
					lastLeftTime = now
				}
				if now-lastLeftTime > hideDelay {
					gui.SetDockHidden(true, dock.DockX, dock.DockY, now) // animate from dock→hide
				}
			}
		}
	}
}

// approach moves cur towards target by at most step, returning the new value.
func approach(cur, target, step int) int {
	switch {
	case cur < target:
		if cur+step >= target {
			return target
		}
		return cur + step
	case cur > target:
		if cur-step <= target {
			return target
		}
		return cur - step
	}
	return target
}

// abs returns the absolute value of x.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// lerp linearly interpolates between a and b by t (0..1).
func lerp(a, b int, t float32) int {
	return a + int(float32(b-a)*t)
}

// cursorInWindow reports whether (cx,cy) is within the window rect.
func cursorInWindow(cx, cy, wx, wy, ww, wh int) bool {
	return cx >= wx && cx < wx+ww && cy >= wy && cy < wy+wh
}

// cursorNearEdge reports whether the cursor is close to the docked edge.
func cursorNearEdge(cx, cy int, edge store.Edge, dx, dy, ww, wh, threshold, workX, workY, workW, workH int) bool {
	switch edge {
	case store.EdgeTop:
		return cy <= workY+threshold && cx >= dx && cx < dx+ww
	case store.EdgeBottom:
		return cy >= workY+workH-threshold && cx >= dx && cx < dx+ww
	case store.EdgeLeft:
		return cx <= workX+threshold && cy >= dy && cy < dy+wh
	case store.EdgeRight:
		return cx >= workX+workW-threshold && cy >= dy && cy < dy+wh
	}
	return false
}
