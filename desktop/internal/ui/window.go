package ui

import (
	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/graydovee/todo-manager/desktop/internal/platform"
)

// WindowSpec describes how to create and drive a secondary window (dialog,
// popup, tool window, ...). It encapsulates all the boilerplate that used to be
// inlined per-window: HWND capture, the one-shot "first frame" setup (owner
// relation, centre, activate), and the event-loop skeleton.
type WindowSpec struct {
	// Title is the native window title.
	Title string
	// Width and Height are the initial window size in dp. Leave 0 to skip
	// setting a size (Gio picks a default).
	Width, Height unit.Dp
	// Decorated controls native window chrome. Dialogs keep decorations
	// (title bar + close button); borderless widgets set this to false.
	Decorated bool

	// Owner is the native handle of the parent window. When non-zero the
	// window is made an owned child via SetupChildWindow so Win32 guarantees
	// it renders above its owner regardless of TOPMOST state. Leave 0 for a
	// standalone window.
	Owner platform.Handle

	// OnReady is invoked exactly once, on the first FrameEvent after the native
	// handle becomes available. This is the correct place to apply any
	// platform-specific setup that must run after the window is fully
	// initialised (doing it on ViewEvent deadlocks). For an owned dialog this
	// typically just calls platform.SetupChildWindow(hwnd, spec.Owner).
	OnReady func(hwnd platform.Handle, w *app.Window)

	// OnEvent processes per-frame logic and should return a value to emit (empty
	// string = no result yet). When it returns a non-empty value the window is
	// closed and the value is sent on the result channel. It is called on every
	// FrameEvent with a fresh gtx, BEFORE Render is called, so it can handle
	// button clicks that influence the frame. The window parameter lets the
	// callback close the window itself via Perform(ActionClose).
	OnEvent func(gtx layout.Context, w *app.Window) string

	// Render draws the window body every frame.
	Render func(gtx layout.Context)
}

// RunWindow creates a secondary window from spec, runs its event loop on the
// calling goroutine (which blocks until the window is destroyed), and returns a
// channel that receives exactly one string: the value returned by OnEvent when
// it decided to close, or "" if the user closed the window directly (native X
// button). The channel is closed immediately after the value is sent.
//
// The caller decides how to consume the channel: block on it, select on it, or
// spawn a goroutine. Because this call blocks, it must NOT run on a goroutine
// that drives another Gio window's event loop.
func RunWindow(spec WindowSpec) <-chan string {
	resultCh := make(chan string, 1)

	w := &app.Window{}
	w.Option(app.Title(spec.Title))
	if spec.Width > 0 && spec.Height > 0 {
		w.Option(app.Size(spec.Width, spec.Height))
	}
	w.Option(app.Decorated(spec.Decorated))

	var (
		hwnd      platform.Handle
		gotHwnd   bool
		ready     bool
		closing   bool
		finalVal  string
	)

	var ops op.Ops
	for {
		e := w.Event()
		switch e := e.(type) {
		case app.ViewEvent:
			// Capture the native handle once. On Windows this is the HWND.
			h := platform.ExtractHandle(e)
			if h != 0 && !gotHwnd {
				gotHwnd = true
				hwnd = platform.Handle(h)
			}
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)

			// One-shot platform setup on the first frame after the handle
			// arrives. Doing it here (not on ViewEvent) avoids the
			// synchronous-message deadlocks that plague early Win32 calls.
			if !ready && gotHwnd {
				ready = true
				if spec.OnReady != nil {
					spec.OnReady(hwnd, w)
				}
			}

			// Per-frame logic. Once a result is produced we stop processing
			// events but keep rendering so the window never appears frozen
			// while the close completes.
			if !closing && spec.OnEvent != nil {
				if v := spec.OnEvent(gtx, w); v != "" {
					finalVal = v
					closing = true
				}
			}

			// Always paint so the window stays responsive while closing.
			if spec.Render != nil {
				spec.Render(gtx)
			}
			e.Frame(gtx.Ops)

		case app.DestroyEvent:
			resultCh <- finalVal
			close(resultCh)
			return resultCh
		}
	}
}
