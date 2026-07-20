// Package platform abstracts the native window operations the desktop client
// needs (always-on-top, click-through lock, positioning, work-area and cursor
// queries). The primary target is Windows; non-Windows platforms link a stub
// implementation so the rest of the app compiles.
package platform

// Platform is the abstraction over native window control. All methods are safe
// to call from any goroutine; implementations that require main-thread/HWND
// affinity route the call through the Fyne native-window driver.
type Platform interface {
	// SetAlwaysOnTop pins the window above all others (or releases it).
	SetAlwaysOnTop(top bool)
	// MoveWindow repositions the window's top-left to (x, y) in screen coords.
	MoveWindow(x, y int)
	// WindowGeometry returns the current window's top-left and pixel size.
	WindowGeometry() (x, y, w, h int)
	// WorkArea returns the work area of the monitor the window is on. The work
	// area excludes the taskbar.
	WorkArea() (x, y, w, h int)
	// CursorPos returns the current cursor position in screen coords.
	CursorPos() (x, y int)
	// SetLock toggles click-through + translucent overlay mode (the app's
	// "lock" feature). When locked, the window cannot receive pointer events.
	SetLock(locked bool)
	// Minimize hides the window to the taskbar.
	Minimize()
}

// New constructs the platform-specific Platform implementation. The window
// argument is the fyne.Window being controlled; it is type-asserted to
// driver.NativeWindow inside the Windows implementation to gain HWND access.
// On unsupported platforms a no-op stub is returned.
func New(window any) Platform {
	return newPlatform(window)
}
