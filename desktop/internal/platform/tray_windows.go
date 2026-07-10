//go:build windows

package platform

import (
	"github.com/graydovee/todo-manager/desktop/internal/platform/windows"
)

// RunTray starts the tray icon + its message loop. Must run on a dedicated
// goroutine (not the Gio main goroutine). It blocks until the tray is removed.
//
// invalidate is called on every tray command to wake up a blocking GUI event
// loop (Gio's w.Event() blocks; w.Invalidate() unblocks it). It may be nil.
func RunTray(topMost, locked bool, cmds chan TrayCmd, invalidate func()) {
	winCmds := make(chan windows.TrayCmd, 8)
	done := make(chan struct{})
	// Bridge native tray commands to the platform channel, waking the GUI loop
	// for each so it can drain the channel.
	go func() {
		for c := range winCmds {
			switch c {
			case windows.TrayToggleLock:
				cmds <- TrayToggleLock
			case windows.TrayToggleTopMost:
				cmds <- TrayToggleTopMost
			case windows.TrayQuit:
				cmds <- TrayQuit
				close(cmds)
				windows.QuitTray()
				return
			}
			if invalidate != nil {
				invalidate()
			}
		}
		close(done)
	}()
	windows.RunTray(topMost, locked, winCmds)
	<-done
}

// SetTrayTopMost updates the tray menu checkbox state.
func SetTrayTopMost(on bool) {
	windows.SetTrayTopMost(on)
}

// SetTrayLock updates the tray menu lock checkbox state.
func SetTrayLock(on bool) {
	windows.SetTrayLock(on)
}
