//go:build !windows

package platform

import "os"

// NewController returns a no-op controller on platforms whose native window
// integration is not yet implemented (macOS, Linux). The app still runs; only
// top-most / click-through are unavailable.
func NewController(_ Handle) Controller {
	return Noop{}
}

// CenterAndTopMost is a no-op on unsupported platforms.
func CenterAndTopMost(_ Handle) {}

// CenterWindow is a no-op on unsupported platforms.
func CenterWindow(_ Handle) {}

// SetDialogOwner is a no-op on unsupported platforms.
func SetDialogOwner(_, _ Handle) {}

// ActivateWindow is a no-op on unsupported platforms.
func ActivateWindow(_ Handle) {}

// SetLogFile is a no-op on platforms without a native platform layer.
func SetLogFile(_ *os.File) {}
