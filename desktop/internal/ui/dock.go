package ui

import (
	"time"

	"github.com/graydovee/todo-manager/desktop/internal/store"
)

// dockLoop is the dock-poll goroutine. It samples the platform's cursor
// position and window geometry at a fixed cadence (every ~80ms) and feeds them
// into the store's DockState to decide whether the window should auto-hide.
//
// Auto-hide triggers when: docked && !dragging && !locked && sidePanelHidden.
// When the user moves the cursor outside the window for hideDelayMs, the window
// slides off to its hide position. When the cursor touches the peek edge, it
// slides back. The slide itself is time-driven (ease-out) so the wall-clock
// duration is exact.
func (a *App) dockLoop() {
	const poll = 80 * time.Millisecond
	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopDock:
			return
		case <-ticker.C:
			a.dockTick()
		}
	}
}

// dockTick performs one sample of the dock state machine. It runs on the poll
// goroutine; platform calls (Win32) are thread-safe so we don't need fyne.Do
// here.
func (a *App) dockTick() {
	// Conditions for auto-hide: docked + not dragging + not locked + side hidden.
	snap := a.State.Dock.Snapshot()
	if !snap.Docked {
		return
	}
	if a.State.Locked {
		return
	}

	curX, curY := a.platform.CursorPos()
	winX, winY, _, _ := a.platform.WindowGeometry()
	now := time.Now().UnixMilli()

	switch {
	case snap.Animating:
		a.stepAnimation(snap, now)
	case shouldHide(&snap, curX, curY, winX, winY):
		// Begin hide only after the hide-delay has elapsed.
		if a.cursorLeftSince == 0 {
			a.cursorLeftSince = now
		}
		if now-a.cursorLeftSince >= int64(a.hideDelayMs()) {
			a.startHide(snap, winX, winY, now)
		}
	default:
		// Cursor is over the window — cancel any pending hide.
		a.cursorLeftSince = 0
		if snap.Hidden && !snap.Animating {
			a.startShow(snap, winX, winY, now)
		}
	}
}

// stepAnimation advances an in-flight slide by one tick using an ease-out curve.
func (a *App) stepAnimation(snap store.DockSnapshot, nowMs int64) {
	durMs := int64(a.animMs())
	if durMs <= 0 {
		durMs = 500
	}
	elapsed := nowMs - snap.AnimStartMs
	if elapsed >= durMs {
		// Snap to target.
		a.platform.MoveWindow(snap.AnimTargetX, snap.AnimTargetY)
		a.State.Dock.Lock()
		a.State.Dock.Animating = false
		if snap.AnimTargetX == snap.HideX && snap.AnimTargetY == snap.HideY {
			a.State.Dock.Hidden = true
		} else {
			a.State.Dock.Hidden = false
		}
		a.State.Dock.Unlock()
		return
	}
	t := float64(elapsed) / float64(durMs)
	eased := 1 - (1-t)*(1-t) // ease-out quad
	x := lerp(snap.AnimStartX, snap.AnimTargetX, eased)
	y := lerp(snap.AnimStartY, snap.AnimTargetY, eased)
	a.platform.MoveWindow(x, y)
}

// startHide kicks off the hide animation via the store's SetHidden helper.
func (a *App) startHide(_ store.DockSnapshot, curX, curY int, nowMs int64) {
	a.State.Dock.SetHidden(true, curX, curY, nowMs)
}

// startShow kicks off the show animation.
func (a *App) startShow(_ store.DockSnapshot, curX, curY int, nowMs int64) {
	a.State.Dock.SetHidden(false, curX, curY, nowMs)
}

// shouldHide reports whether the cursor is outside the window geometry so the
// auto-hide timer should run.
func shouldHide(snap *store.DockSnapshot, curX, curY, winX, winY int) bool {
	if snap.Dragging {
		return false
	}
	if curX >= winX && curX <= winX+snap.WinW && curY >= winY && curY <= winY+snap.WinH {
		return false
	}
	return true
}

// animMs returns the configured slide animation duration (default 500ms).
func (a *App) animMs() int {
	a.State.Lock()
	defer a.State.Unlock()
	if a.State.Config.Dock.AnimMs > 0 {
		return a.State.Config.Dock.AnimMs
	}
	return 500
}

// hideDelayMs returns the configured hide-delay (default 600ms).
func (a *App) hideDelayMs() int {
	a.State.Lock()
	defer a.State.Unlock()
	if a.State.Config.Dock.HideDelayMs > 0 {
		return a.State.Config.Dock.HideDelayMs
	}
	return 600
}

// lerp is a linear interpolation helper used by the ease-out curve.
func lerp(a, b int, t float64) int {
	return a + int(float64(b-a)*t)
}
