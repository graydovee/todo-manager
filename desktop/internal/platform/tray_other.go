//go:build !windows

package platform

// RunTray is a no-op on platforms without a native tray implementation yet.
// It blocks until TrayQuit is sent on cmds (so the caller's goroutine lifetime
// matches a real tray on other platforms).
func RunTray(_, _ bool, cmds chan TrayCmd, invalidate func()) {
	for range cmds {
		if invalidate != nil {
			invalidate()
		}
	}
}

// SetTrayTopMost is a no-op on unsupported platforms.
func SetTrayTopMost(_ bool) {}

// SetTrayLock is a no-op on unsupported platforms.
func SetTrayLock(_ bool) {}
