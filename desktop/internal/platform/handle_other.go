//go:build !windows

package platform

import (
	"gioui.org/app"
)

// ExtractHandle returns the native window handle. Unsupported on non-Windows
// builds for now (returns 0); macOS will use AppKitViewEvent later.
func ExtractHandle(e app.ViewEvent) uintptr {
	_ = e
	return 0
}
