//go:build windows

package platform

import (
	"gioui.org/app"
)

// ExtractHandle pulls the native HWND from a Gio ViewEvent on Windows.
func ExtractHandle(e app.ViewEvent) uintptr {
	if w, ok := e.(app.Win32ViewEvent); ok {
		return w.HWND
	}
	return 0
}
