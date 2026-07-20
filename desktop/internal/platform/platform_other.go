//go:build !windows

package platform

// stubPlatform is a no-op Platform for non-Windows platforms (Linux/macOS dev
// builds). The nativeHWND-specific features (click-through lock, topmost) are
// not available; the rest of the app remains usable.
type stubPlatform struct{}

func newPlatform(_ any) Platform {
	return &stubPlatform{}
}

func (s *stubPlatform) SetAlwaysOnTop(bool) {}
func (s *stubPlatform) MoveWindow(int, int) {}
func (s *stubPlatform) WindowGeometry() (int, int, int, int) {
	return 0, 0, 0, 0
}
func (s *stubPlatform) WorkArea() (int, int, int, int) {
	return 0, 0, 1920, 1080
}
func (s *stubPlatform) CursorPos() (int, int) {
	return 0, 0
}
func (s *stubPlatform) SetLock(bool) {}
func (s *stubPlatform) Minimize()    {}
