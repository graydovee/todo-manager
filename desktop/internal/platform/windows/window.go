//go:build windows

package windows

import (
	"syscall"
	"unsafe"
)

// Win32 constants for window geometry.
const (
	MONITOR_DEFAULTTONEAREST = 0x00000002
)

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

var (
	procGetWindowRect    = modUser32Ex.NewProc("GetWindowRect")
	procMoveWindow       = modUser32Ex.NewProc("MoveWindow")
	procMonitorFromWin   = modUser32Ex.NewProc("MonitorFromWindow")
	procGetMonitorInfo   = modUser32Ex.NewProc("GetMonitorInfoW")
	procShowWindow       = modUser32Ex.NewProc("ShowWindow")
)

// Win32 ShowWindow commands.
const (
	SW_MINIMIZE = 6
)

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

// MoveWindow repositions and resizes the window in one call (repaint=true).
func MoveWindow(hwnd syscall.Handle, x, y, w, h int) {
	_, _, _ = procMoveWindow.Call(uintptr(hwnd), uintptr(x), uintptr(y), uintptr(w), uintptr(h), 1)
}

// WorkArea returns the work area (excluding taskbar) of the monitor nearest the
// window, as (x, y, width, height).
func WorkArea(hwnd syscall.Handle) (x, y, w, h int) {
	hmon, _, _ := procMonitorFromWin.Call(uintptr(hwnd), MONITOR_DEFAULTTONEAREST)
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
	ret, _, _ := procGetCursor.Call(uintptr(unsafe.Pointer(&pt)))
	if ret == 0 {
		return 0, 0
	}
	return int(pt.X), int(pt.Y)
}

// CenterAndTopMost centres the window on the primary monitor's work area and
// makes it top-most. It calls Win32 directly (no goroutine/queue) so it is safe
// to invoke from a dialog's own goroutine.
func CenterAndTopMost(hwnd syscall.Handle) {
	if hwnd == 0 {
		return
	}
	centerOnWorkArea(hwnd)
	_ = SetWindowPos(hwnd, HWND_TOPMOST, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE)
}

// CenterWindow centres the window on the monitor's work area (no top-most).
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
	MoveWindow(hwnd, x, y, w, h)
}

// Minimize minimises the window.
func Minimize(hwnd syscall.Handle) {
	if hwnd == 0 {
		return
	}
	_, _, _ = procShowWindow.Call(uintptr(hwnd), SW_MINIMIZE)
}
