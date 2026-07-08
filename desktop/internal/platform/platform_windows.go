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

// SetLogFile routes platform diagnostic output to f.
func SetLogFile(f *os.File) {
	win.SetLogFile(f)
}

// Compile-time assertion that the windows controller satisfies the interface.
var _ Controller = (*win.Controller)(nil)
