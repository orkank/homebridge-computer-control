package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// StatusDotState: Online (green), Sending (orange), Error (red)
type StatusDotState int

const (
	StatusDotWaiting StatusDotState = iota
	StatusDotSending
	StatusDotOnline
	StatusDotError
)

// statusDotWidget shows a colored dot with optional label.
type statusDotWidget struct {
	circle *canvas.Circle
	label  *widget.Label
	state  StatusDotState
}

func newStatusDotWidget() *statusDotWidget {
	c := canvas.NewCircle(color.Gray{Y: 0x99})
	c.Resize(fyne.NewSize(10, 10))
	l := widget.NewLabel("Waiting...")
	l.TextStyle = fyne.TextStyle{}
	s := &statusDotWidget{circle: c, label: l, state: StatusDotWaiting}
	s.updateColors()
	return s
}

func (s *statusDotWidget) SetState(state StatusDotState) {
	s.state = state
	s.updateColors()
	switch state {
	case StatusDotOnline:
		s.label.SetText("Connected")
	case StatusDotSending:
		s.label.SetText("Sending...")
	case StatusDotError:
		s.label.SetText("Disconnected")
	default:
		s.label.SetText("Waiting...")
	}
	s.label.Refresh()
	s.circle.Refresh()
}

func (s *statusDotWidget) updateColors() {
	var c color.Color
	switch s.state {
	case StatusDotOnline:
		c = color.NRGBA{R: 0x34, G: 0xc7, B: 0x59, A: 0xff} // Green #34c759
	case StatusDotSending:
		c = color.NRGBA{R: 0xff, G: 0x95, B: 0x00, A: 0xff} // Orange #ff9500
	case StatusDotError:
		c = color.NRGBA{R: 0xff, G: 0x3b, B: 0x30, A: 0xff} // Red #ff3b30
	default:
		c = color.NRGBA{R: 0x98, G: 0x98, B: 0x9b, A: 0xff} // Gray #98989b
	}
	s.circle.FillColor = c
}

func (s *statusDotWidget) Object() fyne.CanvasObject {
	return container.NewHBox(s.circle, s.label)
}
