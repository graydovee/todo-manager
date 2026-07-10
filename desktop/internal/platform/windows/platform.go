//go:build windows

package windows

import (
	"fmt"
	"os"
	"sync"
	"syscall"
)

// logWriter is the shared diagnostic sink; main sets it via SetLogFile.
var (
	logMu   sync.Mutex
	logFile *os.File
)

// SetLogFile routes platform diagnostic output to the same file the main
// binary uses (desktop.log).
func SetLogFile(f *os.File) {
	logMu.Lock()
	defer logMu.Unlock()
	logFile = f
}

// logf writes a diagnostic line tagged with [platform].
func logf(format string, args ...any) {
	msg := fmt.Sprintf("[platform] "+format, args...)
	fmt.Fprintln(os.Stderr, msg)
	logMu.Lock()
	defer logMu.Unlock()
	if logFile != nil {
		fmt.Fprintln(logFile, msg)
	}
}

// Controller implements platform.Controller for Windows using Win32 calls.
// All Win32 mutations are routed through the single package-level OS thread
// (RunOnOSThread) because Win32 windows are thread-affine and several APIs
// synchronously deliver window messages that must not run on Gio's window
// goroutine. Reads (geometry, work area, cursor) are direct calls that do not
// deliver messages and are safe on the caller's goroutine.
type Controller struct {
	hwnd syscall.Handle

	mu        sync.Mutex
	curLocked bool
	applied   bool // whether curLocked reflects the real window state
}

// New returns a Controller bound to the given HWND (from app.Win32ViewEvent).
func New(hwnd syscall.Handle) *Controller {
	logf("windows.New(hwnd=%v)", hwnd)
	return &Controller{hwnd: hwnd}
}

// SetTopMost places the window at the top of the z-order.
func (c *Controller) SetTopMost(topmost bool) {
	if c.hwnd == 0 {
		return
	}
	insertAfter := HWND_NOTOPMOST
	if topmost {
		insertAfter = HWND_TOPMOST
	}
	hwnd := c.hwnd
	RunOnOSThread(func() {
		err := SetWindowPos(hwnd, insertAfter, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE)
		logf("SetTopMost(%v) done err=%v", topmost, err)
	})
}

// HideFromTaskbar makes the window a tool window so it disappears from the
// taskbar and Alt+Tab while keeping the tray icon as the only presence.
func (c *Controller) HideFromTaskbar() {
	if c.hwnd == 0 {
		return
	}
	hwnd := c.hwnd
	RunOnOSThread(func() {
		style := GetWindowLongPtr(hwnd, GWL_EXSTYLE)
		SetWindowLongPtr(hwnd, GWL_EXSTYLE, style|WS_EX_TOOLWINDOW)
		logf("HideFromTaskbar done")
	})
}

// WindowGeometry returns the current position and size synchronously. This
// reads directly (GetWindowRect does not deliver messages) so it is safe to
// call from any goroutine and returns an immediate value.
func (c *Controller) WindowGeometry() (x, y, w, h int) {
	if c.hwnd == 0 {
		return 0, 0, 0, 0
	}
	return GetWindowRect(c.hwnd)
}

// MoveWindow repositions the window asynchronously via the OS thread.
func (c *Controller) MoveWindow(x, y, w, h int) {
	if c.hwnd == 0 {
		return
	}
	if w == 0 || h == 0 {
		_, _, cw, ch := GetWindowRect(c.hwnd)
		if w == 0 {
			w = cw
		}
		if h == 0 {
			h = ch
		}
	}
	finalW, finalH := w, h
	hwnd := c.hwnd
	RunOnOSThread(func() {
		MoveWindowFn(hwnd, x, y, finalW, finalH)
	})
}

// MoveWindowSync repositions the window synchronously on the calling goroutine.
// MoveWindow (user32) sends WM_WINDOWPOSCHANGING/WM_PAINT synchronously; this
// is only safe from goroutines that do NOT hold Gio's render context (e.g. the
// dock-poll/animation goroutine). Used per animation frame for smooth slides.
func (c *Controller) MoveWindowSync(x, y, w, h int) {
	if c.hwnd == 0 {
		return
	}
	if w == 0 || h == 0 {
		_, _, cw, ch := GetWindowRect(c.hwnd)
		if w == 0 {
			w = cw
		}
		if h == 0 {
			h = ch
		}
	}
	MoveWindowFn(c.hwnd, x, y, w, h)
}

// WorkArea returns the work area of the monitor nearest the window.
func (c *Controller) WorkArea() (x, y, w, h int) {
	if c.hwnd == 0 {
		return 0, 0, 0, 0
	}
	return WorkArea(c.hwnd)
}

// CursorPos returns the cursor position in screen coordinates.
func (c *Controller) CursorPos() (x, y int) {
	return CursorPos()
}

// Minimize minimises the window asynchronously via the OS thread.
func (c *Controller) Minimize() {
	if c.hwnd == 0 {
		return
	}
	hwnd := c.hwnd
	RunOnOSThread(func() {
		Minimize(hwnd)
	})
}

// SetLock toggles click-through + translucency. Idempotent: if the requested
// state already matches what was last applied, it returns immediately.
func (c *Controller) SetLock(locked bool) {
	if c.hwnd == 0 {
		return
	}
	c.mu.Lock()
	if c.applied && c.curLocked == locked {
		c.mu.Unlock()
		logf("SetLock(%v) skip (already applied)", locked)
		return
	}
	c.curLocked = locked
	c.applied = true
	c.mu.Unlock()

	hwnd := c.hwnd
	RunOnOSThread(func() {
		style := GetWindowLongPtr(hwnd, GWL_EXSTYLE)
		logf("SetLock exStyle=0x%x", style)
		if locked {
			style |= WS_EX_LAYERED | WS_EX_TRANSPARENT
			SetWindowLongPtr(hwnd, GWL_EXSTYLE, style)
			_ = SetLayeredWindowAttributes(hwnd, 0xC7)
			_ = EnableBlurBehind(hwnd, true)
			// Locking implies top-most.
			_ = SetWindowPos(hwnd, HWND_TOPMOST, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE)
		} else {
			if style&(WS_EX_LAYERED|WS_EX_TRANSPARENT) != 0 {
				style &^= WS_EX_TRANSPARENT
				SetWindowLongPtr(hwnd, GWL_EXSTYLE, style)
				_ = SetLayeredWindowAttributes(hwnd, 0xFF)
				_ = EnableBlurBehind(hwnd, false)
			}
		}
		logf("SetLock(%v) done", locked)
	})
}
