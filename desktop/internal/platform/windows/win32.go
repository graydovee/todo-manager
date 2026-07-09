//go:build windows

package windows

import (
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// Win32 constants used for top-most and click-through layered windows.
const (
	GWL_EXSTYLE        = int32(-20)
	GWLP_HWNDPARENT    = int32(-8)
	WS_EX_LAYERED      = 0x00080000
	WS_EX_TRANSPARENT  = 0x00000020
	WS_EX_NOACTIVATE   = 0x08000000
	SWP_NOSIZE         = 0x0001
	SWP_NOMOVE         = 0x0002
	SWP_NOACTIVATE     = 0x0010
	HWND_TOPMOST       = ^uintptr(0) // (HWND)-1
	HWND_NOTOPMOST     = ^uintptr(1) // (HWND)-2
	LWA_ALPHA          = 0x00000002
)

var (
	modUser32  = syscall.NewLazyDLL("user32.dll")
	modDwmapi  = syscall.NewLazyDLL("dwmapi.dll")
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")

	procSetWindowLongPtrW   = modUser32.NewProc("SetWindowLongPtrW")
	procGetWindowLongPtrW   = modUser32.NewProc("GetWindowLongPtrW")
	procSetWindowPos        = modUser32.NewProc("SetWindowPos")
	procSetLayeredWindowAtt = modUser32.NewProc("SetLayeredWindowAttributes")
	procBringWindowToTop    = modUser32.NewProc("BringWindowToTop")
	procSetForegroundWindow = modUser32.NewProc("SetForegroundWindow")
	// DWM blur-behind gives a translucent frosted look; optional and may fail.
	procDwmEnableBlurBehindWindow = modDwmapi.NewProc("DwmEnableBlurBehindWindow")

	// osThread serialises Win32 window mutations that synchronously deliver
	// window messages (SetWindowLongPtrW, MoveWindow, ...). Calling those from
	// Gio's FrameEvent goroutine deadlocks against Gio's window message pump, so
	// they are routed to this dedicated thread instead.
	osThreadOnce sync.Once
	osThreadCh   chan func()
)

// RunOnOSThread executes fn on the dedicated OS thread (which has
// LockOSThread). Win32 windows are thread-affine and several APIs send
// synchronous messages, so they must run off Gio's goroutines.
func RunOnOSThread(fn func()) {
	osThreadOnce.Do(func() {
		osThreadCh = make(chan func(), 64)
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

// SetWindowLongPtr sets a window attribute.
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

// DWM_BLURBEHIND parameters for DwmEnableBlurBehindWindow.
type DWM_BLURBEHIND struct {
	DwFlags                uint32
	FEnable                bool
	HRgnBlur               syscall.Handle
	FTransitionOnMaximized bool
}

const (
	DWM_BB_ENABLE       = 0x00000001
	DWM_BB_BLURREGION   = 0x00000002
	DWM_BB_TRANSITIONONMAXIMIZED = 0x00000004
)

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

