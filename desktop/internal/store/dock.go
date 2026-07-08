package store

import "sync"

// Edge identifies a screen edge the window docked to.
type Edge int

const (
	EdgeNone Edge = iota
	EdgeTop
	EdgeBottom
	EdgeLeft
	EdgeRight
)

// DockState tracks the window's docking (edge-snapping) and auto-hide state.
// It is guarded by its own mutex because the dock poll goroutine updates it.
type DockState struct {
	mu sync.Mutex

	Docked bool
	Edge   Edge
	Hidden bool

	// Visible position (where the window sits when fully shown). This follows
	// the user's along-edge drag, so it is NOT necessarily centred.
	DockX, DockY int
	// Hidden position (where the window slides to when auto-hidden). Derived
	// from DockX/DockY + edge direction.
	HideX, HideY int

	// Window size captured at dock time.
	WinW, WinH int

	// Work area captured at dock time (for clamping along-edge movement).
	WorkX, WorkY, WorkW, WorkH int

	// Dragging is true while the user is dragging the title bar; suppresses
	// auto-hide.
	Dragging bool

	// Animating + animation start/target for smooth slide in/out. The animation
	// is time-driven: AnimStartMs is when it began, and the loop computes
	// progress as elapsed/duration, so the wall-clock duration is exact.
	Animating   bool
	AnimStartX  int
	AnimStartY  int
	AnimTargetX int
	AnimTargetY int
	AnimStartMs int64 // monotonic-ish start (UnixMilli)
}

// Lock locks the dock state mutex.
func (d *DockState) Lock() { d.mu.Lock() }

// Unlock unlocks the dock state mutex.
func (d *DockState) Unlock() { d.mu.Unlock() }

// Reset clears the dock state (e.g. when undocking by dragging away).
func (d *DockState) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Docked = false
	d.Edge = EdgeNone
	d.Hidden = false
	d.Animating = false
}

// SetDock records the dock edge and the current visible position. The hide
// position is computed from the visible position + edge direction.
func (d *DockState) SetDock(edge Edge, dockX, dockY, winW, winH, workX, workY, workW, workH int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	const peek = 4 // px visible when hidden
	d.Docked = true
	d.Edge = edge
	d.Hidden = false
	d.Animating = false
	d.DockX = dockX
	d.DockY = dockY
	d.WinW = winW
	d.WinH = winH
	d.WorkX, d.WorkY, d.WorkW, d.WorkH = workX, workY, workW, workH
	d.HideX, d.HideY = hidePos(edge, dockX, dockY, winW, winH, peek)
}

// UpdateDockPos moves the visible position along the docked edge and recomputes
// the hide position. Used during drag while docked (constrained to the edge).
func (d *DockState) UpdateDockPos(x, y int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	const peek = 4
	// Clamp along-edge position within the work area.
	switch d.Edge {
	case EdgeTop, EdgeBottom:
		d.DockX = clampInt(x, d.WorkX, d.WorkX+d.WorkW-d.WinW)
	case EdgeLeft, EdgeRight:
		d.DockY = clampInt(y, d.WorkY, d.WorkY+d.WorkH-d.WinH)
	}
	d.HideX, d.HideY = hidePos(d.Edge, d.DockX, d.DockY, d.WinW, d.WinH, peek)
	d.Hidden = false
	d.Animating = false
}

// SetDragging marks the drag-in-progress flag.
func (d *DockState) SetDragging(v bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Dragging = v
}

// SetHidden toggles the hidden flag and starts a time-driven animation from
// curX/curY to the hide or dock position.
func (d *DockState) SetHidden(v bool, curX, curY int, nowMs int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Hidden = v
	d.Animating = true
	d.AnimStartX = curX
	d.AnimStartY = curY
	d.AnimStartMs = nowMs
	if v {
		d.AnimTargetX, d.AnimTargetY = d.HideX, d.HideY
	} else {
		d.AnimTargetX, d.AnimTargetY = d.DockX, d.DockY
	}
}

// Snapshot returns a copy of the dock state safe for read-only inspection.
func (d *DockState) Snapshot() DockSnapshot {
	d.mu.Lock()
	defer d.mu.Unlock()
	return DockSnapshot{
		Docked:      d.Docked,
		Edge:        d.Edge,
		Hidden:      d.Hidden,
		DockX:       d.DockX,
		DockY:       d.DockY,
		HideX:       d.HideX,
		HideY:       d.HideY,
		WinW:        d.WinW,
		WinH:        d.WinH,
		WorkX:       d.WorkX,
		WorkY:       d.WorkY,
		WorkW:       d.WorkW,
		WorkH:       d.WorkH,
		Dragging:    d.Dragging,
		Animating:   d.Animating,
		AnimStartX:  d.AnimStartX,
		AnimStartY:  d.AnimStartY,
		AnimTargetX: d.AnimTargetX,
		AnimTargetY: d.AnimTargetY,
		AnimStartMs: d.AnimStartMs,
	}
}

// DockSnapshot is an immutable copy of DockState fields.
type DockSnapshot struct {
	Docked      bool
	Edge        Edge
	Hidden      bool
	DockX       int
	DockY       int
	HideX       int
	HideY       int
	WinW        int
	WinH        int
	WorkX       int
	WorkY       int
	WorkW       int
	WorkH       int
	Dragging    bool
	Animating   bool
	AnimStartX  int
	AnimStartY  int
	AnimTargetX int
	AnimTargetY int
	AnimStartMs int64
}

// hidePos computes the off-screen position for a given dock position + edge.
func hidePos(edge Edge, dockX, dockY, winW, winH, peek int) (int, int) {
	switch edge {
	case EdgeTop:
		return dockX, dockY - winH + peek
	case EdgeBottom:
		return dockX, dockY + winH - peek
	case EdgeLeft:
		return dockX - winW + peek, dockY
	case EdgeRight:
		return dockX + winW - peek, dockY
	}
	return dockX, dockY
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
