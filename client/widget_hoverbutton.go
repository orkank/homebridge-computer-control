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

// hoverButton is a small text button that turns Accent Blue with white text on hover.
type hoverButton struct {
	widget.BaseWidget
	text    string
	onTap   func()
	hovered bool

	bg    *canvas.Rectangle
	label *canvas.Text
}

func newHoverButton(text string, onTap func()) *hoverButton {
	b := &hoverButton{text: text, onTap: onTap}
	b.ExtendBaseWidget(b)
	return b
}

func (b *hoverButton) CreateRenderer() fyne.WidgetRenderer {
	b.bg = canvas.NewRectangle(color.Transparent)
	b.bg.CornerRadius = 5

	// Use theme-aware text: light mode = dark text, dark mode = light text (window frame is dark)
	labelColor := macLightFg
	if fyneApp != nil && fyneApp.Settings().ThemeVariant() == theme.VariantDark {
		labelColor = macDarkFg
	}
	b.label = canvas.NewText(b.text, labelColor)
	b.label.TextSize = 12
	b.label.Alignment = fyne.TextAlignCenter

	content := container.NewStack(b.bg, container.NewCenter(b.label))

	return &hoverButtonRenderer{btn: b, content: content}
}

// Hoverable
func (b *hoverButton) MouseIn(*desktop.MouseEvent) {
	b.hovered = true
	b.Refresh()
}
func (b *hoverButton) MouseOut() {
	b.hovered = false
	b.Refresh()
}
func (b *hoverButton) MouseMoved(*desktop.MouseEvent) {}

// Tappable
func (b *hoverButton) Tapped(*fyne.PointEvent) {
	if b.onTap != nil {
		b.onTap()
	}
}

// Cursor
func (b *hoverButton) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

type hoverButtonRenderer struct {
	btn     *hoverButton
	content *fyne.Container
}

func (r *hoverButtonRenderer) Layout(size fyne.Size) {
	r.content.Resize(size)
}

func (r *hoverButtonRenderer) MinSize() fyne.Size {
	textMin := r.btn.label.MinSize()
	return fyne.NewSize(textMin.Width+24, textMin.Height+10)
}

func (r *hoverButtonRenderer) Refresh() {
	if r.btn.hovered {
		r.btn.bg.FillColor = macAccent
		r.btn.label.Color = color.White
	} else {
		r.btn.bg.FillColor = color.Transparent
		// Theme-aware: dark mode = light text (visible on dark window frame)
		if fyneApp != nil && fyneApp.Settings().ThemeVariant() == theme.VariantDark {
			r.btn.label.Color = macDarkFg
		} else {
			r.btn.label.Color = macLightFg
		}
	}
	r.btn.bg.Refresh()
	r.btn.label.Refresh()
}

func (r *hoverButtonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.content}
}

func (r *hoverButtonRenderer) Destroy() {}
