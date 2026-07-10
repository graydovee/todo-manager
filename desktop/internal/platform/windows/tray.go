//go:build windows

package windows

import (
	_ "embed"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/graydovee/todo-manager/desktop/internal/i18n"
)

//go:embed icon.ico
var appIconICO []byte

// TrayCmd is an action chosen from the tray menu.
type TrayCmd int

const (
	TrayToggleLock TrayCmd = iota
	TrayToggleTopMost
	TrayQuit
)

// trayState carries the shared flags the menu reflects (checked state of the
// lock and top-most items) and the command channel. It is read from the tray's
// window procedure and written from other goroutines, hence the mutex.
type trayState struct {
	mu      sync.Mutex
	topMost bool
	locked  bool
	cmds    chan TrayCmd
	hwnd    syscall.Handle
}

var (
	globalTray *trayState
)

// RunTray installs a tray icon and runs its message loop on the calling
// goroutine. The caller MUST run this on a dedicated goroutine, never the Gio
// main goroutine. The goroutine locks itself to its OS thread because Win32
// windows have thread affinity (the message loop must run on the creating
// thread). It blocks until the icon is removed.
//
// topMost is the initial checked state of the "Always on top" item.
func RunTray(topMost, locked bool, cmds chan TrayCmd) {
	// Pin this goroutine to its OS thread: the message window is thread-affine.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	s := &trayState{
		topMost: topMost,
		locked:  locked,
		cmds:    cmds,
	}
	globalTray = s

	className, _ := syscall.UTF16PtrFromString("TodoManagerTray")
	wc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		LpfnWndProc:   syscall.NewCallback(trayWndProc),
		LpszClassName: className,
	}
	atom, _, _ := procRegisterClass.Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		// registration failure is non-fatal; tray just won't appear.
		return
	}

	windowName, _ := syscall.UTF16PtrFromString("TodoManagerTrayWindow")
	hwnd, _, _ := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		0, // WS_OVERLAPPED hidden; message-only because hWndParent = HWND_MESSAGE (-3)
		0, 0, 0, 0,
		^uintptr(2), // HWND_MESSAGE = (HWND)-3
		0, 0, 0, 0,
	)
	if hwnd == 0 {
		return
	}
	s.hwnd = syscall.Handle(hwnd)

	// Install the tray icon (custom app icon, falling back to the default).
	icon, _ := LoadIconFromICO(appIconICO)
	if icon == 0 {
		icon, _ = LoadIcon(IDI_APPLICATION)
	}
	tip, _ := syscall.UTF16FromString(i18n.T("common.appName"))
	nid := NOTIFYICONDATA{
		CbSize:           uint32(unsafe.Sizeof(NOTIFYICONDATA{})),
		HWnd:             s.hwnd,
		UID:              1,
		UFlags:           NIF_MESSAGE | NIF_ICON | NIF_TIP,
		UCallbackMessage: WM_TRAYICON,
		HIcon:            icon,
	}
	copy(nid.SzTip[:], tip)
	_ = ShellNotifyIcon(NIM_ADD, &nid)

	// Message loop for the tray's hidden window only.
	var msg MSG
	for {
		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		_, _, _ = procTranslateMsg.Call(uintptr(unsafe.Pointer(&msg)))
		_, _, _ = procDispatchMsg.Call(uintptr(unsafe.Pointer(&msg)))
	}

	// Cleanup.
	_ = ShellNotifyIcon(NIM_DELETE, &nid)
	_, _, _ = procDestroyWindow.Call(hwnd)
}

// trayWndProc handles messages for the hidden tray window. Right-click shows
// the popup menu; the chosen command is sent on the state channel. WM_CLOSE
// ends the message loop (via WM_DESTROY -> PostQuitMessage).
func trayWndProc(hwnd syscall.Handle, msg uint32, wparam, lparam uintptr) uintptr {
	switch msg {
	case WM_TRAYICON:
		switch lparam {
		case WM_RBUTTONUP, WM_LBUTTONUP:
			showTrayMenu(hwnd)
		}
		return 0
	case WM_COMMAND:
		cmd := int(wparam)
		s := globalTray
		if s == nil {
			return 0
		}
		switch cmd {
		case trayMenuLock:
			select {
			case s.cmds <- TrayToggleLock:
			default:
			}
		case trayMenuTopMost:
			select {
			case s.cmds <- TrayToggleTopMost:
			default:
			}
		case trayMenuQuit:
			select {
			case s.cmds <- TrayQuit:
			default:
			}
		}
		return 0
	case WM_DESTROY:
		_, _, _ = procPostQuit.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wparam, lparam)
	return r
}

// QuitTray asks the tray window to close. Safe to call from any goroutine:
// PostMessage is thread-safe and wakes the tray's GetMessage loop.
func QuitTray() {
	s := globalTray
	if s == nil || s.hwnd == 0 {
		return
	}
	_, _, _ = procPostMessage.Call(uintptr(s.hwnd), 0x0010 /* WM_CLOSE */, 0, 0)
}

// showTrayMenu builds and displays the context menu.
func showTrayMenu(hwnd syscall.Handle) {
	s := globalTray
	if s == nil {
		return
	}
	s.mu.Lock()
	topMost := s.topMost
	locked := s.locked
	s.mu.Unlock()

	menu, _, _ := procCreateMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	menuAddItem(menu, trayMenuLock, i18n.T("tray.lock"), true, locked)
	menuAddItem(menu, trayMenuTopMost, i18n.T("tray.topMost"), true, topMost)
	menuAddSeparator(menu)
	menuAddItem(menu, trayMenuQuit, i18n.T("tray.quit"), false, false)

	// TrackPopupMenu must be preceded by SetForegroundWindow on the owner,
	// otherwise the menu won't dismiss when clicking elsewhere.
	_, _, _ = procSetForegroundWindow.Call(uintptr(hwnd))

	var pt POINT
	_, _, _ = procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	cmd, _, _ := procTrackPopupMenu.Call(
		menu,
		TPM_RIGHTBUTTON|TPM_BOTTOMALIGN|TPM_RETURNCMD,
		uintptr(pt.X), uintptr(pt.Y), 0, uintptr(hwnd), 0,
	)
	if cmd != 0 {
		_, _, _ = procPostMessage.Call(uintptr(hwnd), WM_COMMAND, cmd, 0)
	}
}

func menuAddItem(menu uintptr, id int, label string, checkable, checked bool) {
	text, _ := syscall.UTF16PtrFromString(label)
	flags := uint32(0)
	if checked {
		flags |= 0x0008 // MF_CHECKED
	}
	_, _, _ = procAppendMenu.Call(menu, uintptr(flags), uintptr(id), uintptr(unsafe.Pointer(text)))
}

func menuAddSeparator(menu uintptr) {
	_, _, _ = procAppendMenu.Call(menu, 0x00000800 /*MF_SEPARATOR*/, 0, 0) //nolint:gomnd
}

// SetTrayTopMost updates the checkbox state shown on the next menu open.
func SetTrayTopMost(on bool) {
	if globalTray == nil {
		return
	}
	globalTray.mu.Lock()
	defer globalTray.mu.Unlock()
	globalTray.topMost = on
}

// SetTrayLock updates the lock checkbox state shown on the next menu open.
func SetTrayLock(on bool) {
	if globalTray == nil {
		return
	}
	globalTray.mu.Lock()
	defer globalTray.mu.Unlock()
	globalTray.locked = on
}
