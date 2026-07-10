//go:build windows

package windows

import (
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// Win32 window-style constants used for top-most, click-through, and ownership.
const (
	GWL_EXSTYLE       = int32(-20)
	GWLP_HWNDPARENT   = int32(-8)
	WS_EX_LAYERED     = 0x00080000
	WS_EX_TRANSPARENT = 0x00000020
	WS_EX_TOOLWINDOW  = 0x00000080
	SWP_NOSIZE        = 0x0001
	SWP_NOMOVE        = 0x0002
	SWP_NOACTIVATE    = 0x0010
	HWND_TOPMOST      = ^uintptr(0) // (HWND)-1
	HWND_NOTOPMOST    = ^uintptr(1) // (HWND)-2
	LWA_ALPHA         = 0x00000002
)

// modUser32 is the single user32.dll handle for the whole package. All proc
// bindings derive from it (the OS dedups LazyDLL loads, but a single source is
// far less confusing than scattered duplicate declarations).
var modUser32 = syscall.NewLazyDLL("user32.dll")

var (
	procSetWindowLongPtrW   = modUser32.NewProc("SetWindowLongPtrW")
	procGetWindowLongPtrW   = modUser32.NewProc("GetWindowLongPtrW")
	procSetWindowPos        = modUser32.NewProc("SetWindowPos")
	procSetLayeredWindowAtt = modUser32.NewProc("SetLayeredWindowAttributes")
	procBringWindowToTop    = modUser32.NewProc("BringWindowToTop")
	procSetForegroundWindow = modUser32.NewProc("SetForegroundWindow")
	procGetWindowRect       = modUser32.NewProc("GetWindowRect")
	procMoveWindow          = modUser32.NewProc("MoveWindow")
	procMonitorFromWindow   = modUser32.NewProc("MonitorFromWindow")
	procGetMonitorInfo      = modUser32.NewProc("GetMonitorInfoW")
	procShowWindow          = modUser32.NewProc("ShowWindow")
	procGetCursorPos        = modUser32.NewProc("GetCursorPos")
)

// ShowWindow commands.
const (
	SW_MINIMIZE = 6
)

// Monitor info constants.
const (
	MONITOR_DEFAULTTONEAREST = 0x00000002
)

// osThread is the single dedicated OS thread for all Win32 window mutations.
// Win32 windows are thread-affine and several APIs (SetWindowLongPtrW,
// MoveWindow, SetWindowPos, ...) synchronously deliver window messages
// (WM_STYLECHANGED, WM_WINDOWPOSCHANGING, WM_PAINT). Calling them from Gio's
// window goroutine deadlocks against Gio's own message pump, so every
// message-delivering mutation is routed here. A single thread also avoids
// cross-thread deadlocks that arose when two parallel dedicated threads each
// held their own message-sending queue.
var (
	osThreadOnce sync.Once
	osThreadCh   chan func()
)

// RunOnOSThread executes fn on the dedicated OS thread (created lazily on first
// call). All Win32 calls that synchronously deliver window messages MUST go
// through this. The channel is buffered (256) so brief bursts of mutations
// (e.g. drag events) do not block senders.
func RunOnOSThread(fn func()) {
	osThreadOnce.Do(func() {
		osThreadCh = make(chan func(), 256)
		go func() {
			runtime.LockOSThread()
			for f := range osThreadCh {
				f()
			}
		}()
	})
	osThreadCh <- fn
}

// GetWindowLongPtr retrieves a window attribute.
func GetWindowLongPtr(hwnd syscall.Handle, index int32) uintptr {
	ret, _, _ := procGetWindowLongPtrW.Call(uintptr(hwnd), uintptr(index))
	return ret
}

// SetWindowLongPtr sets a window attribute. The index is an int32 (negative
// values such as GWL_EXSTYLE/GWLP_HWNDPARENT are common).
func SetWindowLongPtr(hwnd syscall.Handle, index int32, value uintptr) uintptr {
	ret, _, _ := procSetWindowLongPtrW.Call(uintptr(hwnd), uintptr(index), value)
	return ret
}

// SetWindowPos changes z-order, size and position. flags is a bitmask.
func SetWindowPos(hwnd syscall.Handle, insertAfter uintptr, x, y, w, h int32, flags uint32) error {
	ret, _, err := procSetWindowPos.Call(uintptr(hwnd), insertAfter, uintptr(x), uintptr(y), uintptr(w), uintptr(h), uintptr(flags))
	if ret == 0 {
		return err
	}
	return nil
}

// SetLayeredWindowAttributes sets the per-window alpha. crKey=0 with LWA_ALPHA.
func SetLayeredWindowAttributes(hwnd syscall.Handle, alpha uint8) error {
	ret, _, err := procSetLayeredWindowAtt.Call(uintptr(hwnd), 0, uintptr(alpha), uintptr(LWA_ALPHA))
	if ret == 0 {
		return err
	}
	return nil
}

// SetWindowOwner establishes an owner-owned relationship: the dialog (child)
// will always render above its owner regardless of either window's TOPMOST
// state. This is the reliable way to keep a dialog above a top-most main window.
// It sets GWLP_HWNDPARENT via SetWindowLongPtrW.
func SetWindowOwner(child, owner syscall.Handle) {
	if child == 0 || owner == 0 {
		return
	}
	SetWindowLongPtr(child, GWLP_HWNDPARENT, uintptr(owner))
}

// ActivateWindow brings the window to the top of the z-order within its
// (top-most/non-top-most) band and attempts to set it as the foreground window.
// Used for dialogs so they receive keyboard focus.
func ActivateWindow(hwnd syscall.Handle) {
	if hwnd == 0 {
		return
	}
	procSetForegroundWindow.Call(uintptr(hwnd))
	procBringWindowToTop.Call(uintptr(hwnd))
}

// RECT is a Win32 rectangle in screen coordinates.
type RECT struct {
	Left, Top, Right, Bottom int32
}

// MONITORINFO holds monitor information including the work area.
type MONITORINFO struct {
	CbSize    uint32
	RcMonitor RECT
	RcWork    RECT
	DwFlags   uint32
}

// POINT is a Win32 point (used for cursor position and menu coordinates).
type POINT struct {
	X, Y int32
}

// GetWindowRect returns the window's outer rectangle (x, y, width, height) in
// screen coordinates.
func GetWindowRect(hwnd syscall.Handle) (x, y, w, h int) {
	var rc RECT
	ret, _, _ := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
	if ret == 0 {
		return 0, 0, 0, 0
	}
	return int(rc.Left), int(rc.Top), int(rc.Right - rc.Left), int(rc.Bottom - rc.Top)
}

// MoveWindowFn repositions and resizes the window in one call (repaint=true).
func MoveWindowFn(hwnd syscall.Handle, x, y, w, h int) {
	_, _, _ = procMoveWindow.Call(uintptr(hwnd), uintptr(x), uintptr(y), uintptr(w), uintptr(h), 1)
}

// WorkArea returns the work area (excluding taskbar) of the monitor nearest the
// window, as (x, y, width, height).
func WorkArea(hwnd syscall.Handle) (x, y, w, h int) {
	hmon, _, _ := procMonitorFromWindow.Call(uintptr(hwnd), MONITOR_DEFAULTTONEAREST)
	if hmon == 0 {
		return 0, 0, 0, 0
	}
	var mi MONITORINFO
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	ret, _, _ := procGetMonitorInfo.Call(hmon, uintptr(unsafe.Pointer(&mi)))
	if ret == 0 {
		return 0, 0, 0, 0
	}
	return int(mi.RcWork.Left), int(mi.RcWork.Top),
		int(mi.RcWork.Right - mi.RcWork.Left), int(mi.RcWork.Bottom - mi.RcWork.Top)
}

// CursorPos returns the cursor position in screen coordinates.
func CursorPos() (x, y int) {
	var pt POINT
	ret, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if ret == 0 {
		return 0, 0
	}
	return int(pt.X), int(pt.Y)
}

// CenterWindow centres the window on the monitor's work area.
func CenterWindow(hwnd syscall.Handle) {
	if hwnd == 0 {
		return
	}
	centerOnWorkArea(hwnd)
}

// centerOnWorkArea moves hwnd to the centre of its monitor's work area.
func centerOnWorkArea(hwnd syscall.Handle) {
	wx, wy, workW, workH := WorkArea(hwnd)
	_, _, w, h := GetWindowRect(hwnd)
	if w == 0 || h == 0 {
		w, h = 480, 240
	}
	x := wx + (workW-w)/2
	y := wy + (workH-h)/2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	MoveWindowFn(hwnd, x, y, w, h)
}

// Minimize minimises the window via ShowWindow(SW_MINIMIZE).
func Minimize(hwnd syscall.Handle) {
	if hwnd == 0 {
		return
	}
	_, _, _ = procShowWindow.Call(uintptr(hwnd), SW_MINIMIZE)
}

// DWM blur-behind types and proc for the optional frosted-glass look.
var procDwmEnableBlurBehindWindow = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmEnableBlurBehindWindow")

const (
	DWM_BB_ENABLE              = 0x00000001
	DWM_BB_BLURREGION          = 0x00000002
	DWM_BB_TRANSITIONONMAXIMIZED = 0x00000004
)

// DWM_BLURBEHIND parameters for DwmEnableBlurBehindWindow.
type DWM_BLURBEHIND struct {
	DwFlags                uint32
	FEnable                bool
	HRgnBlur               syscall.Handle
	FTransitionOnMaximized bool
}

// EnableBlurBehind attempts to enable DWM blur-behind. It is best-effort; an
// error is returned (and ignored by callers) when unsupported.
func EnableBlurBehind(hwnd syscall.Handle, enable bool) error {
	bb := DWM_BLURBEHIND{
		DwFlags: DWM_BB_ENABLE,
		FEnable: enable,
	}
	_, _, err := procDwmEnableBlurBehindWindow.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&bb)))
	return err
}
