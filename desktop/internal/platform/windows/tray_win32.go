//go:build windows

package windows

import (
	"syscall"
	"unsafe"
)

// Win32 constants for the shell notification icon (tray).
const (
	NIM_ADD         = 0x00000000
	NIM_DELETE      = 0x00000002
	NIM_MODIFY      = 0x00000001
	NIF_MESSAGE     = 0x00000001
	NIF_ICON        = 0x00000002
	NIF_TIP         = 0x00000004
	WM_USER         = 0x0400
	WM_APP          = 0x8000
	WM_DESTROY      = 0x0002
	WM_COMMAND      = 0x0111
	WM_LBUTTONUP    = 0x0202
	WM_RBUTTONUP    = 0x0205
	WM_LBUTTONDBLCLK = 0x0203
	TPM_RIGHTBUTTON = 0x0002
	TPM_BOTTOMALIGN = 0x0020
	TPM_RETURNCMD   = 0x0100

	IDI_APPLICATION = 32512

	// WS_EX_TOOLWINDOW: a tool window has a smaller title bar and does NOT appear
	// in the taskbar or Alt+Tab. Used to keep the widget out of the taskbar while
	// the tray icon remains the only presence.
	WS_EX_TOOLWINDOW = 0x00000080

	// Tray callback message id, chosen in the WM_APP range to avoid clashes.
	WM_TRAYICON = WM_APP + 1

	// Menu item ids.
	trayMenuUnlock   = 1001
	trayMenuTopMost  = 1002
	trayMenuShow     = 1003
	trayMenuHide     = 1004
	trayMenuQuit     = 1005
)

var (
	modShell32  = syscall.NewLazyDLL("shell32.dll")
	modGdi32    = syscall.NewLazyDLL("gdi32.dll")
	modUser32Ex = syscall.NewLazyDLL("user32.dll")

	procShellNotifyIcon = modShell32.NewProc("Shell_NotifyIconW")
	procLoadIcon        = modUser32Ex.NewProc("LoadIconW")
	procCreateMenu      = modUser32Ex.NewProc("CreatePopupMenu")
	procAppendMenu      = modUser32Ex.NewProc("AppendMenuW")
	procTrackPopupMenu  = modUser32Ex.NewProc("TrackPopupMenu")
	procDestroyMenu     = modUser32Ex.NewProc("DestroyMenu")
	procPostMessage     = modUser32Ex.NewProc("PostMessageW")
	procGetMessage      = modUser32Ex.NewProc("GetMessageW")
	procDefWindowProc   = modUser32Ex.NewProc("DefWindowProcW")
	procRegisterClass   = modUser32Ex.NewProc("RegisterClassExW")
	procCreateWindowEx  = modUser32Ex.NewProc("CreateWindowExW")
	procDestroyWindow   = modUser32Ex.NewProc("DestroyWindow")
	procTranslateMsg    = modUser32Ex.NewProc("TranslateMessage")
	procDispatchMsg     = modUser32Ex.NewProc("DispatchMessageW")
	procPostQuit        = modUser32Ex.NewProc("PostQuitMessage")
	procSetForeground   = modUser32Ex.NewProc("SetForegroundWindow")
	procGetCursor       = modUser32Ex.NewProc("GetCursorPos")
	procLookupIconId    = modUser32Ex.NewProc("LookupIconIdFromDirectoryEx")
	procCreateIconRes   = modUser32Ex.NewProc("CreateIconFromResourceEx")
)

// Icon-loading constants.
const (
	LR_DEFAULTSIZE = 0x00000040
)

// NOTIFYICONDATA for the current Windows version (dwInfoFields via cbSize).
type NOTIFYICONDATA struct {
	CbSize           uint32
	HWnd             syscall.Handle
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            syscall.Handle
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UTimeout         uint32
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	// GuidItem and balloon fields omitted; cbSize selects the v3 layout below.
	HBalloonIcon syscall.Handle
}

// WNDCLASSEX describes a window class for the hidden message window.
type WNDCLASSEX struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     syscall.Handle
	HIcon         syscall.Handle
	HCursor       syscall.Handle
	HbrBackground syscall.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       syscall.Handle
}

// POINT is a Win32 point.
type POINT struct {
	X, Y int32
}

// MSG is a Win32 message.
type MSG struct {
	HWnd    syscall.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

// ShellNotifyIcon wraps Shell_NotifyIconW.
func ShellNotifyIcon(dwMessage uint32, data *NOTIFYICONDATA) error {
	r1, _, err := procShellNotifyIcon.Call(uintptr(dwMessage), uintptr(unsafe.Pointer(data)))
	if r1 == 0 {
		return err
	}
	return nil
}

// LoadIcon loads a stock icon (e.g. IDI_APPLICATION).
func LoadIcon(id uintptr) (syscall.Handle, error) {
	r1, _, err := procLoadIcon.Call(0, id)
	if r1 == 0 {
		return 0, err
	}
	return syscall.Handle(r1), nil
}

// LoadIconFromICO decodes an embedded .ico file (its full bytes) into an HICON.
// It uses LookupIconIdFromDirectoryEx to pick the best image then
// CreateIconFromResourceEx to realise it. Falls back to (0, error) on failure.
func LoadIconFromICO(ico []byte) (syscall.Handle, error) {
	if len(ico) == 0 {
		return 0, nil
	}
	// Pick the most appropriate icon entry for the default size.
	id, _, _ := procLookupIconId.Call(uintptr(unsafe.Pointer(&ico[0])), 1, 0)
	if id == 0 {
		return 0, nil
	}
	// CreateIconFromResourceEx expects a pointer to the icon image bits for the
	// chosen entry. The directory layout is: ICONDIR (6 bytes) + entries; the
	// returned id is a byte offset within the file to the image bits.
	if int(id) >= len(ico) {
		return 0, nil
	}
	bits := ico[id:]
	h, _, err := procCreateIconRes.Call(
		uintptr(unsafe.Pointer(&bits[0])),
		uintptr(len(bits)),
		1, // fIcon = TRUE
		0x00030000, // LR_DEFAULTSIZE-ish version mask
		0, 0,
	)
	if h == 0 {
		return 0, err
	}
	return syscall.Handle(h), nil
}
