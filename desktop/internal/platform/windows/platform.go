//go:build windows

package windows

import (
	"fmt"
	"os"
	"runtime"
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

// winTask is a window-style mutation to run on the dedicated win thread.
type winTask struct {
	fn      func() error
	summary string
}

// Controller implements platform.Controller for Windows using Win32 calls.
// All Win32 mutations run on a dedicated OS thread (Win32 windows and their
// styles are thread-affine, and several calls synchronously deliver window
// messages such as WM_STYLECHANGED that must not run on Gio's window thread).
type Controller struct {
	hwnd syscall.Handle

	tasks chan winTask

	mu        sync.Mutex
	curLocked bool
	applied   bool // whether curLocked reflects the real window state
}

// New returns a Controller bound to the given HWND (from app.Win32ViewEvent).
// It launches a dedicated thread to serialise all Win32 style mutations.
func New(hwnd syscall.Handle) *Controller {
	logf("windows.New(hwnd=%v)", hwnd)
	c := &Controller{
		hwnd:  hwnd,
		tasks: make(chan winTask, 16),
	}
	go c.winThread()
	return c
}

// winThread owns a dedicated OS thread for running Win32 window mutations.
func (c *Controller) winThread() {
	runtime.LockOSThread()
	for t := range c.tasks {
		err := t.fn()
		logf("%s done err=%v", t.summary, err)
	}
}

// exec submits a task and returns immediately. Mutations are fire-and-forget;
// the UI repaints via Invalidate once the caller returns.
func (c *Controller) exec(summary string, fn func() error) {
	select {
	case c.tasks <- winTask{fn: fn, summary: summary}:
	default:
		logf("%s dropped (queue full)", summary)
	}
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
	c.exec(fmt.Sprintf("SetTopMost(%v)", topmost), func() error {
		return SetWindowPos(c.hwnd, insertAfter, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE)
	})
}

// HideFromTaskbar makes the window a tool window so it disappears from the
// taskbar and Alt+Tab while keeping the tray icon as the only presence.
func (c *Controller) HideFromTaskbar() {
	if c.hwnd == 0 {
		return
	}
	c.exec("HideFromTaskbar", func() error {
		style := GetWindowLongPtr(c.hwnd, GWL_EXSTYLE)
		SetWindowLongPtr(c.hwnd, GWL_EXSTYLE, style|WS_EX_TOOLWINDOW)
		return nil
	})
}

// WindowGeometry returns the current position and size synchronously. It reads
// directly (not via the task queue) because callers need an immediate value.
func (c *Controller) WindowGeometry() (x, y, w, h int) {
	if c.hwnd == 0 {
		return 0, 0, 0, 0
	}
	return GetWindowRect(c.hwnd)
}

// MoveWindow repositions the window asynchronously via the win thread. Calling
// user32!MoveWindow synchronously on the UI goroutine deadlocks against Gio's
// window thread (MoveWindow sends WM_WINDOWPOSCHANGING/WM_PAINT synchronously).
// The async queue is drained fast enough for smooth dragging.
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
	c.exec("MoveWindow", func() error {
		MoveWindow(c.hwnd, x, y, finalW, finalH)
		return nil
	})
}

// MoveWindowSync repositions the window synchronously on the calling goroutine.
// It bypasses the win-thread queue so the position is applied immediately —
// necessary for smooth animation where each frame must land before the next.
// Safe to call from the dock poll goroutine (which does not hold Gio's render
// context, so there is no deadlock risk).
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
	MoveWindow(c.hwnd, x, y, w, h)
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

	c.exec(fmt.Sprintf("SetLock(%v)", locked), func() error {
		style := GetWindowLongPtr(c.hwnd, GWL_EXSTYLE)
		logf("SetLock exStyle=0x%x", style)
		if locked {
			style |= WS_EX_LAYERED | WS_EX_TRANSPARENT
			SetWindowLongPtr(c.hwnd, GWL_EXSTYLE, style)
			_ = SetLayeredWindowAttributes(c.hwnd, 0xC7)
			_ = EnableBlurBehind(c.hwnd, true)
			// Locking implies top-most.
			_ = SetWindowPos(c.hwnd, HWND_TOPMOST, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE)
		} else {
			if style&(WS_EX_LAYERED|WS_EX_TRANSPARENT) != 0 {
				style &^= WS_EX_TRANSPARENT
				SetWindowLongPtr(c.hwnd, GWL_EXSTYLE, style)
				_ = SetLayeredWindowAttributes(c.hwnd, 0xFF)
				_ = EnableBlurBehind(c.hwnd, false)
			}
		}
		return nil
	})
}
