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

// CenterAndTopMost centres the window on the primary monitor and makes it
// top-most. Calls Win32 directly (no queue).
func CenterAndTopMost(h Handle) {
	win.CenterAndTopMost(syscall.Handle(h))
}

// CenterWindow centres the window on its monitor's work area (no top-most).
// Run on the dedicated OS thread because MoveWindow synchronously delivers
// window messages that would deadlock Gio's window goroutine.
func CenterWindow(h Handle) {
	win.RunOnOSThread(func() {
		win.CenterWindow(syscall.Handle(h))
	})
}

// SetDialogOwner makes child an owned window of owner, so child always renders
// above owner regardless of either window's TOPMOST state. No-op if either is 0.
// Run on the dedicated OS thread because SetWindowLongPtrW synchronously
// delivers window messages that would deadlock Gio's window goroutine.
func SetDialogOwner(child, owner Handle) {
	win.RunOnOSThread(func() {
		win.SetWindowOwner(syscall.Handle(child), syscall.Handle(owner))
	})
}

// ActivateWindow brings the window to the front and sets it as foreground.
// Run on the dedicated OS thread for the same reason as SetDialogOwner.
func ActivateWindow(h Handle) {
	win.RunOnOSThread(func() {
		win.ActivateWindow(syscall.Handle(h))
	})
}

// SetLogFile routes platform diagnostic output to f.
func SetLogFile(f *os.File) {
	win.SetLogFile(f)
}

// Compile-time assertion that the windows controller satisfies the interface.
var _ Controller = (*win.Controller)(nil)
