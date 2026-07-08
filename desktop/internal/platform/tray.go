package platform

// TrayCmd is an action chosen from the tray menu.
type TrayCmd int

const (
	TrayUnlock TrayCmd = iota
	TrayToggleTopMost
	TrayQuit
)
