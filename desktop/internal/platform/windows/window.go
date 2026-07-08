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
