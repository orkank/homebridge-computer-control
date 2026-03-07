package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// macTheme provides a modern macOS-style theme (Light/Dark, accent color).
// Uses default system fonts for cross-platform compatibility.
type macTheme struct {
	base fyne.Theme
}

func newMacTheme() fyne.Theme {
	return &macTheme{base: theme.DefaultTheme()}
}

// ──────────────── Colors ────────────────

// macOS-style colors (Light Mode) — Off-White / Vibrancy inspired
var (
	macLightBg        = color.NRGBA{R: 0xf6, G: 0xf6, B: 0xf6, A: 0xff} // #f6f6f6 off-white
	macLightSidebar   = color.NRGBA{R: 0xf0, G: 0xf0, B: 0xf0, A: 0xff} // slightly tinted sidebar
	macLightFg        = color.NRGBA{R: 0x1d, G: 0x1d, B: 0x1f, A: 0xff} // #1d1d1f
	macLightSecondary = color.NRGBA{R: 0x6e, G: 0x6e, B: 0x73, A: 0xff} // #6e6e73  secondary text
	macLightMuted     = color.NRGBA{R: 0x86, G: 0x86, B: 0x8b, A: 0xff} // #86868b
	macLightInput     = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	macLightBorder    = color.NRGBA{R: 0xd1, G: 0xd1, B: 0xd6, A: 0xff}
	macLightCard      = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
)

// macOS-style colors (Dark Mode)
var (
	macDarkBg        = color.NRGBA{R: 0x1c, G: 0x1c, B: 0x1e, A: 0xff} // #1c1c1e
	macDarkSidebar   = color.NRGBA{R: 0x23, G: 0x23, B: 0x25, A: 0xff}
	macDarkFg        = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	macDarkSecondary = color.NRGBA{R: 0x8e, G: 0x8e, B: 0x93, A: 0xff} // #8e8e93
	macDarkMuted     = color.NRGBA{R: 0x98, G: 0x98, B: 0x9b, A: 0xff} // #98989b
	macDarkInput     = color.NRGBA{R: 0x2c, G: 0x2c, B: 0x2e, A: 0xff} // #2c2c2e
	macDarkBorder    = color.NRGBA{R: 0x38, G: 0x38, B: 0x3a, A: 0xff}
	macDarkCard      = color.NRGBA{R: 0x2c, G: 0x2c, B: 0x2e, A: 0xff}
)

// macOS standard Accent Blue (#007aff)
var macAccent = color.NRGBA{R: 0x00, G: 0x7a, B: 0xff, A: 0xff}

func (t *macTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	light := variant == theme.VariantLight
	switch name {
	case theme.ColorNameBackground:
		if light {
			return macLightBg
		}
		return macDarkBg
	case theme.ColorNameForeground:
		if light {
			return macLightFg
		}
		return macDarkFg
	case theme.ColorNameInputBackground:
		if light {
			return macLightInput
		}
		return macDarkInput
	case theme.ColorNameInputBorder:
		if light {
			return macLightBorder
		}
		return macDarkBorder
	case theme.ColorNameDisabled:
		if light {
			return macLightMuted
		}
		return macDarkMuted
	case theme.ColorNamePlaceHolder:
		if light {
			return macLightMuted
		}
		return macDarkMuted
	case theme.ColorNamePrimary:
		return macAccent
	case theme.ColorNameButton:
		return macAccent
	case theme.ColorNameHeaderBackground:
		if light {
			return macLightCard
		}
		return macDarkCard
	case theme.ColorNameHover:
		if light {
			return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x0c} // very subtle dark hover
		}
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x0c}
	case theme.ColorNamePressed:
		if light {
			return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x18}
		}
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x18}
	case theme.ColorNameFocus:
		return macAccent
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0x00, G: 0x7a, B: 0xff, A: 0x30}
	case theme.ColorNameForegroundOnPrimary:
		return color.White
	case theme.ColorNameForegroundOnError:
		return color.White
	case theme.ColorNameForegroundOnSuccess:
		return color.White
	case theme.ColorNameForegroundOnWarning:
		return color.White
	case theme.ColorNameHyperlink:
		return macAccent
	case theme.ColorNameDisabledButton:
		if light {
			return macLightMuted
		}
		return macDarkMuted
	case theme.ColorNameScrollBar:
		if light {
			return macLightMuted
		}
		return macDarkMuted
	case theme.ColorNameScrollBarBackground:
		return color.Transparent
	case theme.ColorNameMenuBackground:
		if light {
			return macLightCard
		}
		return macDarkCard
	case theme.ColorNameOverlayBackground:
		if light {
			return macLightBg
		}
		return macDarkBg
	case theme.ColorNameSeparator:
		if light {
			return color.NRGBA{R: 0xd1, G: 0xd1, B: 0xd6, A: 0x80} // lighter separator
		}
		return color.NRGBA{R: 0x48, G: 0x48, B: 0x4a, A: 0x80}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x18}
	}
	return t.base.Color(name, variant)
}

func (t *macTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.base.Font(style)
}

func (t *macTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *macTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 8 // tighter, more Apple-like
	case theme.SizeNameInnerPadding:
		return 6
	case theme.SizeNameText:
		return 13
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameScrollBar:
		return 6
	case theme.SizeNameScrollBarSmall:
		return 3
	case theme.SizeNameSeparatorThickness:
		return 0.5
	}
	return t.base.Size(name)
}
