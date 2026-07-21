package ui

import "fyne.io/fyne/v2"

// listSideLayout lays out exactly two children: a fixed-width list column (index
// 0) followed by a side panel (index 1) that absorbs all remaining horizontal
// space. The list column always gets listWidth pixels; the side panel fills the
// rest. This keeps the todo list a constant width regardless of window size,
// while the side panel grows/shrinks with the window.
type listSideLayout struct {
	listWidth float32
}

func newListSideLayout(listWidth float32) fyne.Layout {
	return &listSideLayout{listWidth: listWidth}
}

func (l *listSideLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	// List column: fixed width, full height.
	listSize := fyne.NewSize(l.listWidth, size.Height)
	objects[0].Resize(listSize)
	objects[0].Move(fyne.NewPos(0, 0))
	// Side panel: whatever is left (may be 0 if hidden / window narrow).
	if len(objects) > 1 {
		sideWidth := size.Width - l.listWidth
		if sideWidth < 0 {
			sideWidth = 0
		}
		objects[1].Resize(fyne.NewSize(sideWidth, size.Height))
		objects[1].Move(fyne.NewPos(l.listWidth, 0))
	}
}

func (l *listSideLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	h := float32(0)
	w := l.listWidth
	for i, o := range objects {
		m := o.MinSize()
		if i == 1 { // side panel contributes its own min width only when visible
			if o.Visible() {
				w += m.Width
			}
		}
		if m.Height > h {
			h = m.Height
		}
	}
	return fyne.NewSize(w, h)
}
