//go:build !windows

package i18n

import "os"

// systemLocale returns the locale hint from the environment on non-Windows
// platforms (the LANG variable, e.g. "zh_CN.UTF-8").
func systemLocale() string {
	return os.Getenv("LANG") + "," + os.Getenv("LC_ALL") + "," + os.Getenv("LC_MESSAGES")
}
