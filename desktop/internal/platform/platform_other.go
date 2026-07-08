//go:build !windows

package platform

import "os"

// NewController returns a no-op controller on platforms whose native window
// integration is not yet implemented (macOS, Linux). The app still runs; only
// top-most / click-through are unavailable.
func NewController(_ Handle) Controller {
	return Noop{}
}

// SetLogFile is a no-op on platforms without a native platform layer.
func SetLogFile(_ *os.File) {}
