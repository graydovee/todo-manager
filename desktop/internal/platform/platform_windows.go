//go:build windows

package platform

import (
	"sync"
	"unsafe"

	"fyne.io/fyne/v2/driver"
	"golang.org/x/sys/windows"
)

// Win32 constants.
const (
	wsExLayered             = 0x00080000
	wsExTransparent         = 0x00000020
	hwndTopmost    uintptr  = ^uintptr(0) // HWND_TOPMOST  (-1)
	hwndNoTopmost  uintptr  = ^uintptr(1) // HWND_NOTOPMOST (-2)
	swpNoSize               = 0x0001
	swpNoMove               = 0x0002
	swpNoZOrder             = 0x0004
	swpNoActivate           = 0x0010
	swMinimize              = 6
	lwaAlpha                = 0x00000002
	monitorDefaultToNearest = 0x00000002
)

// gwlExStyle is a var (not const) to avoid uintptr constant overflow (-20).
var gwlExStyle int32 = -20 // GWL_EXSTYLE

var (
	modUser32 = windows.NewLazySystemDLL("user32.dll")
	modGdi32  = windows.NewLazySystemDLL("gdi32.dll")

	procGetWindowRect        = modUser32.NewProc("GetWindowRect")
	procMonitorFromWindow    = modUser32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW      = modUser32.NewProc("GetMonitorInfoW")
	procGetCursorPos         = modUser32.NewProc("GetCursorPos")
	procSetWindowLongPtrW    = modUser32.NewProc("SetWindowLongPtrW")
	procGetWindowLongPtrW    = modUser32.NewProc("GetWindowLongPtrW")
	procShowWindow           = modUser32.NewProc("ShowWindow")
	procSetWindowPos         = modUser32.NewProc("SetWindowPos")
	procSetLayeredWindowAttr = modUser32.NewProc("SetLayeredWindowAttributes")
)

type rect struct {
	Left, Top, Right, Bottom int32
}

type point struct {
	X, Y int32
}

// monitorInfoEx mirrors MONITORINFOEX (we only need rcWork).
type monitorInfo struct {
	CbSize    uint32
	RcMonitor rect
	RcWork    rect
	DwFlags   uint32
	SzDevice  [32]uint16
}

// windowsPlatform implements Platform on Windows via direct Win32 calls. It
// resolves the native HWND once (through Fyne's RunNative) and caches it.
type windowsPlatform struct {
	mu     sync.Mutex
	window any // must satisfy driver.NativeWindow
	hwnd   windows.HWND
	got    bool
}

func newPlatform(window any) Platform {
	return &windowsPlatform{window: window}
}

// resolveHWND extracts the native HWND via Fyne's RunNative callback. The
// callback runs on the Fyne main goroutine; we do not hold p.mu while it
// executes (to avoid deadlock when RunNative re-enters the main goroutine).
func (p *windowsPlatform) resolveHWND() windows.HWND {
	p.mu.Lock()
	if p.got {
		hwnd := p.hwnd
		p.mu.Unlock()
		return hwnd
	}
	p.mu.Unlock()

	nw, ok := p.window.(driver.NativeWindow)
	if !ok {
		return 0
	}
	var hwnd windows.HWND
	nw.RunNative(func(ctx any) {
		// Fyne v2.8 passes the HWND (typed or as a raw pointer) to the callback.
		switch v := ctx.(type) {
		case windows.HWND:
			hwnd = v
		case *windows.HWND:
			if v != nil {
				hwnd = *v
			}
		case uintptr:
			hwnd = windows.HWND(v)
		}
	})

	p.mu.Lock()
	p.hwnd = hwnd
	p.got = true
	p.mu.Unlock()
	return hwnd
}

// SetAlwaysOnTop pins the window above all non-topmost windows.
func (p *windowsPlatform) SetAlwaysOnTop(top bool) {
	hwnd := p.resolveHWND()
	if hwnd == 0 {
		return
	}
	after := hwndNoTopmost
	if top {
		after = hwndTopmost
	}
	_, _, _ = procSetWindowPos.Call(uintptr(hwnd), after, 0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoActivate)
}

// MoveWindow repositions the window to (x, y) without changing size or z-order.
func (p *windowsPlatform) MoveWindow(x, y int) {
	hwnd := p.resolveHWND()
	if hwnd == 0 {
		return
	}
	_, _, _ = procSetWindowPos.Call(uintptr(hwnd), 0,
		uintptr(x), uintptr(y), 0, 0,
		swpNoSize|swpNoZOrder|swpNoActivate)
}

// WindowGeometry returns the current window rect.
func (p *windowsPlatform) WindowGeometry() (x, y, w, h int) {
	hwnd := p.resolveHWND()
	if hwnd == 0 {
		return
	}
	var r rect
	_, _, _ = procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r)))
	return int(r.Left), int(r.Top), int(r.Right - r.Left), int(r.Bottom - r.Top)
}

// WorkArea returns the work area of the monitor the window is on.
func (p *windowsPlatform) WorkArea() (x, y, w, h int) {
	hwnd := p.resolveHWND()
	if hwnd == 0 {
		return
	}
	mon, _, _ := procMonitorFromWindow.Call(uintptr(hwnd), monitorDefaultToNearest)
	if mon == 0 {
		return
	}
	var mi monitorInfo
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	_, _, _ = procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi)))
	return int(mi.RcWork.Left), int(mi.RcWork.Top),
		int(mi.RcWork.Right - mi.RcWork.Left), int(mi.RcWork.Bottom - mi.RcWork.Top)
}

// CursorPos returns the current cursor position in screen coords.
func (p *windowsPlatform) CursorPos() (x, y int) {
	var pt point
	_, _, _ = procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	return int(pt.X), int(pt.Y)
}

// SetLock toggles click-through (WS_EX_TRANSPARENT) and a translucent layered
// overlay. When locked, the window cannot receive pointer events.
func (p *windowsPlatform) SetLock(locked bool) {
	hwnd := p.resolveHWND()
	if hwnd == 0 {
		return
	}
	idx := uintptr(gwlExStyle) // break constant to avoid uintptr overflow
	ex, _, _ := procGetWindowLongPtrW.Call(uintptr(hwnd), idx)
	style := uintptr(ex)
	if locked {
		style |= wsExLayered | wsExTransparent
	} else {
		style &^= wsExTransparent
		style |= wsExLayered // keep layered so we can fade
	}
	_, _, _ = procSetWindowLongPtrW.Call(uintptr(hwnd), idx, style)
	if locked {
		// 0xCC/0xFF = ~80% opacity, glassy translucent look.
		_, _, _ = procSetLayeredWindowAttr.Call(uintptr(hwnd), 0, 0xCC, lwaAlpha)
	} else {
		_, _, _ = procSetLayeredWindowAttr.Call(uintptr(hwnd), 0, 0xFF, lwaAlpha)
	}
}

// Minimize hides the window to the taskbar.
func (p *windowsPlatform) Minimize() {
	hwnd := p.resolveHWND()
	if hwnd == 0 {
		return
	}
	_, _, _ = procShowWindow.Call(uintptr(hwnd), swMinimize)
}

// keep gdi32 reference alive for future paint operations.
var _ = modGdi32
