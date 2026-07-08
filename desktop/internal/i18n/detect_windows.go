//go:build windows

package i18n

import (
	"syscall"
	"unsafe"
)

var (
	modKernel32            = syscall.NewLazyDLL("kernel32.dll")
	procGetUserLocaleName  = modKernel32.NewProc("GetUserDefaultLocaleName")
)

// systemLocale returns the user's default locale name (e.g. "zh-CN") on Windows.
func systemLocale() string {
	buf := make([]uint16, 85) // LOCALE_NAME_MAX_LENGTH
	r, _, _ := procGetUserLocaleName.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}
