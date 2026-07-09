// Package platform abstracts the small set of native window behaviours the app
// needs: always-on-top and lock (top-most + click-through + glassy look). Each
// platform implements Controller via build-tagged files.
package platform

// Controller mutates the OS window. Methods are idempotent. A no-op controller
// is used on platforms without a native implementation yet.
type Controller interface {
	// SetTopMost places the window above all non-topmost windows (true) or
	// restores normal z-order (false). The window still receives input.
	SetTopMost(topmost bool)
	// SetLock makes the window top-most, click-through (pointer events pass to
	// whatever is behind it) and signals the UI to render the glass style.
	// Locking implies top-most; unlocking restores the previous top-most state
	// via a subsequent SetTopMost call from the caller.
	SetLock(locked bool)
	// HideFromTaskbar removes the window from the taskbar and Alt+Tab so the tray
	// icon is the only taskbar presence.
	HideFromTaskbar()

	// WindowGeometry returns the window's position and size (screen coordinates).
	WindowGeometry() (x, y, w, h int)
	// MoveWindow repositions the window to the given screen coordinates, keeping
	// its current size when w/h are zero.
	MoveWindow(x, y, w, h int)
	// MoveWindowSync repositions the window synchronously (bypassing the async
	// win-thread queue) for smooth animation. Only safe from non-UI goroutines.
	MoveWindowSync(x, y, w, h int)
	// WorkArea returns the work area (excluding taskbar) of the monitor nearest
	// the window.
	WorkArea() (x, y, w, h int)
	// CursorPos returns the cursor position in screen coordinates.
	CursorPos() (x, y int)
	// Minimize minimises the window to the taskbar.
	Minimize()
}

// Handle carries the native window identifier once it becomes available.
type Handle uintptr

// Noop is a Controller that does nothing; used on unsupported platforms.
type Noop struct{}

func (Noop) SetTopMost(bool)       {}
func (Noop) SetLock(bool)          {}
func (Noop) HideFromTaskbar()      {}
func (Noop) WindowGeometry() (int, int, int, int) { return 0, 0, 0, 0 }
func (Noop) MoveWindow(int, int, int, int)        {}
func (Noop) MoveWindowSync(int, int, int, int)    {}
func (Noop) WorkArea() (int, int, int, int)       { return 0, 0, 0, 0 }
func (Noop) CursorPos() (int, int)                 { return 0, 0 }
func (Noop) Minimize()                             {}
