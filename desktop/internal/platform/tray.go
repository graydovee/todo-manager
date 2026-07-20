// Package platform — system tray wiring.
//
// The tray exposes the three actions the user can take without the window
// being clickable: unlock (toggle lock), pin on top (toggle topmost), and quit.
// Fyne's desktop.App drives the tray via SetSystemTrayMenu. The menu items are
// kept checkable and re-synced when the app's lock/topmost state changes.
package platform

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// TrayCallbacks are the actions the tray menu can trigger.
type TrayCallbacks struct {
	OnToggleLock    func()
	OnToggleTopMost func()
	OnQuit          func()
}

// Tray wraps a Fyne menu used as the system-tray menu. We keep references to
// the checkable items so we can sync their state from anywhere in the app.
type Tray struct {
	app  fyne.App
	menu *fyne.Menu

	lockItem    *fyne.MenuItem
	topMostItem *fyne.MenuItem
}

// NewTray constructs a tray. SetSystemTrayMenu is called on the desktop.App so
// the menu appears in the OS notification area.
func NewTray(app fyne.App, cb TrayCallbacks) *Tray {
	t := &Tray{app: app}

	t.lockItem = fyne.NewMenuItem("", cb.OnToggleLock)
	t.lockItem.Checked = false
	t.topMostItem = fyne.NewMenuItem("", cb.OnToggleTopMost)
	t.topMostItem.Checked = false

	quitItem := fyne.NewMenuItem("", cb.OnQuit)

	t.menu = fyne.NewMenu("",
		t.lockItem,
		t.topMostItem,
		fyne.NewMenuItemSeparator(),
		quitItem,
	)

	if desk, ok := app.(desktop.App); ok {
		desk.SetSystemTrayMenu(t.menu)
	}
	return t
}

// SetLock updates the lock checkbox label and checked state.
func (t *Tray) SetLock(locked bool, label string) {
	if t == nil || t.lockItem == nil {
		return
	}
	t.lockItem.Label = label
	t.lockItem.Checked = locked
	t.refresh()
}

// SetTopMost updates the top-most checkbox label and checked state.
func (t *Tray) SetTopMost(top bool, label string) {
	if t == nil || t.topMostItem == nil {
		return
	}
	t.topMostItem.Label = label
	t.topMostItem.Checked = top
	t.refresh()
}

// SetQuitLabel sets the quit item label (called when language changes).
func (t *Tray) SetQuitLabel(label string) {
	if t == nil || t.menu == nil || len(t.menu.Items) < 4 {
		return
	}
	// Items: [lock, topmost, separator, quit]
	t.menu.Items[3].Label = label
	t.refresh()
}

func (t *Tray) refresh() {
	if desk, ok := t.app.(desktop.App); ok {
		desk.SetSystemTrayMenu(t.menu)
	}
}
