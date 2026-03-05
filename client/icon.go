package main

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icon.png
var iconBytes []byte

// getAppIcon returns the embedded application icon as a Fyne resource.
func getAppIcon() fyne.Resource {
	return fyne.NewStaticResource("icon.png", iconBytes)
}
