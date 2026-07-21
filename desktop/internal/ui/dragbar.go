package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// dragBar wraps a canvas object and makes the region draggable: pressing and
// dragging the mouse over it moves the whole application window via the
// platform layer. It is used to give the borderless window a draggable title
// bar. The wrapped child (typically the title label) is still rendered on top.
type dragBar struct {
	widget.BaseWidget
	app   *App
	child fyne.CanvasObject

	// Gesture state, captured on the first Dragged event of each drag. All
	// quantities are OS screen pixels (via platform.CursorPos /
	// WindowGeometry) — NOT Fyne canvas DIP — so the math is DPI-correct and
	// unaffected by the window moving under the cursor.
	dragging  bool
	startWX   int // window top-left X in screen pixels at drag start
	startWY   int // window top-left Y in screen pixels at drag start
	startCurX int // cursor X in screen pixels at drag start
	startCurY int // cursor Y in screen pixels at drag start
}

// newDragBar wraps child so the underlying region drags the application window.
func newDragBar(app *App, child fyne.CanvasObject) *dragBar {
	d := &dragBar{app: app, child: child}
	d.ExtendBaseWidget(d)
	return d
}

// Dragged moves the window by the same offset the cursor travelled since the
// drag began. Both the window origin and the cursor position are sampled in OS
// screen pixels (platform.CursorPos / WindowGeometry), so the gesture is
// DPI-correct on HiDPI displays and immune to the moving-reference-frame
// problem (the cursor's screen position is independent of the window's
// position, exactly like a native Win32 caption drag).
func (d *dragBar) Dragged(_ *fyne.DragEvent) {
	if !d.dragging {
		// First event of this gesture: capture the window's screen origin and the
		// cursor's start position, and suppress dock auto-hide while dragging.
		wx, wy, _, _ := d.app.platform.WindowGeometry()
		cx, cy := d.app.platform.CursorPos()
		d.startWX = wx
		d.startWY = wy
		d.startCurX = cx
		d.startCurY = cy
		d.dragging = true
		d.app.State.Dock.SetDragging(true)
	}
	// Re-sample the cursor each event; move the window by the cursor's screen
	// displacement since the gesture started.
	cx, cy := d.app.platform.CursorPos()
	d.app.platform.MoveWindow(d.startWX+(cx-d.startCurX), d.startWY+(cy-d.startCurY))
}

// DragEnd is called when the mouse button is released. We clear the dragging
// flag and re-enable dock auto-hide.
func (d *dragBar) DragEnd() {
	if d.dragging {
		d.dragging = false
		d.app.State.Dock.SetDragging(false)
	}
}

// CreateRenderer renders the wrapped child filling the whole drag region.
func (d *dragBar) CreateRenderer() fyne.WidgetRenderer {
	return &dragBarRenderer{
		bar:     d,
		objects: []fyne.CanvasObject{d.child},
	}
}

type dragBarRenderer struct {
	bar     *dragBar
	objects []fyne.CanvasObject
}

func (r *dragBarRenderer) Layout(size fyne.Size) {
	r.bar.child.Resize(size)
}

func (r *dragBarRenderer) MinSize() fyne.Size {
	return r.bar.child.MinSize()
}

func (r *dragBarRenderer) Refresh() {
	r.bar.child.Refresh()
}

func (r *dragBarRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *dragBarRenderer) Destroy()                     {}
