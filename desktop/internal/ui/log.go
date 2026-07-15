package ui

import "fmt"

// log.go exposes a process-wide, injectable diagnostic sink for the ui package.
// main.go owns the actual log file (desktop.log) and installs a writer via
// SetLogger. The ui package — notably the side window's event-loop goroutine —
// calls Logf to record diagnostics that would otherwise be lost when a non-main
// goroutine panics: the main-goroutine recover in main.go cannot catch panics
// from other goroutines.

// logger is the installed diagnostic sink. It receives an already-formatted
// message (no trailing newline required). It is nil before SetLogger is called;
// Logf is a no-op then so callers need no guard. Once installed it must be safe
// to call from any goroutine.
var logger func(string)

// SetLogger installs the diagnostic sink. main passes in its file-backed writer
// (which holds its own mutex). Intended to be called once at startup.
func SetLogger(f func(string)) { logger = f }

// Logf formats and writes a diagnostic line to the installed sink (if any). It
// is safe for concurrent use from any goroutine.
func Logf(format string, args ...any) {
	if logger == nil {
		return
	}
	logger(fmt.Sprintf(format, args...))
}
