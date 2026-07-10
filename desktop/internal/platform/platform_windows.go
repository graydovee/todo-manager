//go:build windows

package platform

import (
	"os"
	"syscall"

	win "github.com/graydovee/todo-manager/desktop/internal/platform/windows"
)

// NewController builds the Windows controller from a native HWND.
func NewController(handle Handle) Controller {
	return win.New(syscall.Handle(handle))
}

// SetupChildWindow is the standard one-shot initialisation for a secondary
// (child) window such as a dialog. It establishes the Win32 owner-owned
// relationship (so the child always renders above its owner regardless of
// either window's TOPMOST state), centres the child on the monitor's work
// area, and brings it to the foreground for keyboard focus — all as a single
// atomic operation on the dedicated OS thread. No-op if either handle is 0.
func SetupChildWindow(child, owner Handle) {
	c, o := syscall.Handle(child), syscall.Handle(owner)
	win.RunOnOSThread(func() {
		win.SetWindowOwner(c, o)
		win.CenterWindow(c)
		win.ActivateWindow(c)
	})
}

// CenterWindow centres a window on its monitor's work area. Exposed for
// non-standard callers; prefer SetupChildWindow for dialogs.
func CenterWindow(h Handle) {
	hwnd := syscall.Handle(h)
	win.RunOnOSThread(func() {
		win.CenterWindow(hwnd)
	})
}

// SetDialogOwner establishes an owner-owned relationship. Exposed for
// non-standard callers; prefer SetupChildWindow.
func SetDialogOwner(child, owner Handle) {
	c, o := syscall.Handle(child), syscall.Handle(owner)
	win.RunOnOSThread(func() {
		win.SetWindowOwner(c, o)
	})
}

// ActivateWindow brings a window to the front and sets it as foreground.
// Exposed for non-standard callers; prefer SetupChildWindow.
func ActivateWindow(h Handle) {
	hwnd := syscall.Handle(h)
	win.RunOnOSThread(func() {
		win.ActivateWindow(hwnd)
	})
}

// SetLogFile routes platform diagnostic output to f.
func SetLogFile(f *os.File) {
	win.SetLogFile(f)
}

// Compile-time assertion that the windows controller satisfies the interface.
var _ Controller = (*win.Controller)(nil)
