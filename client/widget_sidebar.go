package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// sidebarItem is a transparent, icon+label row that highlights on hover/select.
type sidebarItem struct {
	widget.BaseWidget
	icon     fyne.Resource
	text     string
	selected bool
	hovered  bool
	onTap    func()

	bg       *canvas.Rectangle
	iconImg  *widget.Icon
	label    *widget.RichText
}

func newSidebarItem(text string, icon fyne.Resource, onTap func()) *sidebarItem {
	s := &sidebarItem{
		icon:  icon,
		text:  text,
		onTap: onTap,
	}
	s.ExtendBaseWidget(s)
	return s
}

func (s *sidebarItem) SetSelected(v bool) {
	s.selected = v
	s.Refresh()
}

// ── Renderer ──

func (s *sidebarItem) CreateRenderer() fyne.WidgetRenderer {
	s.bg = canvas.NewRectangle(color.Transparent)
	s.bg.CornerRadius = 6

	s.iconImg = widget.NewIcon(s.icon)

	s.label = widget.NewRichTextWithText(s.text)
	s.label.Segments[0].(*widget.TextSegment).Style = widget.RichTextStyle{
		SizeName: theme_SizeNameText,
	}

	row := container.NewHBox(s.iconImg, s.label)

	return &sidebarItemRenderer{item: s, bg: s.bg, row: row}
}

const theme_SizeNameText = "text" // matches Fyne theme size key

// Hoverable interface
func (s *sidebarItem) MouseIn(*desktop.MouseEvent) {
	s.hovered = true
	s.Refresh()
}
func (s *sidebarItem) MouseOut() {
	s.hovered = false
	s.Refresh()
}
func (s *sidebarItem) MouseMoved(*desktop.MouseEvent) {}

// Tappable interface
func (s *sidebarItem) Tapped(*fyne.PointEvent) {
	if s.onTap != nil {
		s.onTap()
	}
}

// ── renderer ──

type sidebarItemRenderer struct {
	item *sidebarItem
	bg   *canvas.Rectangle
	row  *fyne.Container
}

func (r *sidebarItemRenderer) Layout(size fyne.Size) {
	r.bg.Move(fyne.NewPos(0, 0))
	r.bg.Resize(size)
	r.row.Move(fyne.NewPos(8, 2))
	r.row.Resize(fyne.NewSize(size.Width-16, size.Height-4))
}

func (r *sidebarItemRenderer) MinSize() fyne.Size {
	min := r.row.MinSize()
	return fyne.NewSize(min.Width+16, min.Height+4)
}

func (r *sidebarItemRenderer) Refresh() {
	isDark := fyneApp != nil && fyneApp.Settings().ThemeVariant() == theme.VariantDark
	if r.item.selected {
		// macOS selected sidebar item: subtle blue bg
		r.bg.FillColor = color.NRGBA{R: 0x00, G: 0x7a, B: 0xff, A: 0x1c}
		r.item.label.Segments[0].(*widget.TextSegment).Style.ColorName = "primary"
	} else if r.item.hovered {
		// Light mode: dark overlay; dark mode: light overlay
		if isDark {
			r.bg.FillColor = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x08}
		} else {
			r.bg.FillColor = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x08}
		}
		r.item.label.Segments[0].(*widget.TextSegment).Style.ColorName = ""
	} else {
		r.bg.FillColor = color.Transparent
		r.item.label.Segments[0].(*widget.TextSegment).Style.ColorName = ""
	}
	r.bg.Refresh()
	r.item.label.Refresh()
}

func (r *sidebarItemRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.row}
}

func (r *sidebarItemRenderer) Destroy() {}
