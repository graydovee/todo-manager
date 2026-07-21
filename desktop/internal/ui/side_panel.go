package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/graydovee/todo-manager/desktop/internal/i18n"
)

// sidePanel wraps the body of a side panel mode (detail / manage / create) with
// a small header carrying the mode title and a "<" collapse button. When the
// mode is SideDetail, an Edit/Save toggle button is shown as well.
//
// App owns the sideHost container that physically shows/hides the side panel
// (and widens the window to make room for it); this widget is the visual chrome
// around whatever content body App has mounted.
type sidePanel struct {
	app *App

	root    *fyne.Container
	title   *widget.Label
	collBtn *widget.Button
	editBtn *widget.Button
	body    fyne.CanvasObject
}

// newSidePanel constructs a side panel chrome for the given body. The optional
// edit callback wires up the detail Edit/Save button (nil hides it).
func newSidePanel(app *App, title string, body fyne.CanvasObject, onEdit func()) *sidePanel {
	sp := &sidePanel{app: app, body: body}
	sp.title = widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	sp.collBtn = widget.NewButtonWithIcon("", chevronLeft(), func() {
		app.hideSidePanel()
		app.list.topBar.Refresh()
	})
	sp.collBtn.Importance = widget.LowImportance
	if onEdit != nil {
		sp.editBtn = widget.NewButton(i18n.T("common.edit"), onEdit)
	} else {
		sp.editBtn = widget.NewButton("", nil)
		sp.editBtn.Hide()
	}

	headerRight := container.NewHBox()
	if sp.editBtn != nil && sp.editBtn.Visible() {
		headerRight.Add(sp.editBtn)
	}
	headerRight.Add(sp.collBtn)

	header := newSidePanelHeader(sp.title, headerRight)
	sp.root = container.NewBorder(header, nil, nil, nil, body)
	return sp
}

// sidePanelHeader is a custom widget that renders a hairline under the title
// row, matching the minimalist greyscale aesthetic of the rest of the app.
type sidePanelHeader struct {
	widget.BaseWidget
	left  fyne.CanvasObject
	right fyne.CanvasObject
}

func newSidePanelHeader(left, right fyne.CanvasObject) *sidePanelHeader {
	h := &sidePanelHeader{left: left, right: right}
	h.ExtendBaseWidget(h)
	return h
}

func (h *sidePanelHeader) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.NRGBA{R: 0xFA, G: 0xFA, B: 0xFA, A: 0xFF})
	sep := canvas.NewLine(color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF})
	sep.StrokeWidth = 1
	content := container.NewBorder(nil, nil, h.left, h.right)
	return &sidePanelHeaderRenderer{
		header:  h,
		bg:      bg,
		sep:     sep,
		content: content,
		objects: []fyne.CanvasObject{bg, content, sep},
	}
}

type sidePanelHeaderRenderer struct {
	header  *sidePanelHeader
	bg      *canvas.Rectangle
	sep     *canvas.Line
	content *fyne.Container
	objects []fyne.CanvasObject
}

func (r *sidePanelHeaderRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.content.Resize(size)
	r.content.Move(fyne.NewPos(8, 4))
	r.sep.Resize(fyne.NewSize(size.Width, 1))
	r.sep.Move(fyne.NewPos(0, size.Height-1))
}

func (r *sidePanelHeaderRenderer) MinSize() fyne.Size {
	return r.content.MinSize().Add(fyne.NewSize(16, 8))
}

func (r *sidePanelHeaderRenderer) Refresh() {
	r.bg.Refresh()
	r.content.Refresh()
	r.sep.Refresh()
}

func (r *sidePanelHeaderRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *sidePanelHeaderRenderer) Destroy()                     {}

// SetEditLabel toggles the edit button label between Edit and Save states.
func (sp *sidePanel) SetEditLabel(text string) {
	if sp.editBtn == nil || !sp.editBtn.Visible() {
		return
	}
	sp.editBtn.SetText(text)
}
