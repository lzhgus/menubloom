package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Palette is a theme of Settings: a hue that tints the window and colors
// its accent. Lightness is fixed per role, so every palette reads the same;
// only hue and chroma change, in OKLCH, where equal steps look equal.
type Palette struct {
	Name   string
	Hue    float32
	Tint   float32 // how much of the hue the window takes, 0–1
	Accent float32 // the accent's chroma
}

// Palettes, the first the default.
var Palettes = []Palette{
	{"Apricot", 55, 1, 0.15},
	{"Meadow", 150, 1, 0.13},
	{"Petal", 355, 1, 0.15},
	{"Mist", 250, 0.55, 0.12},
	{"Dusk", 305, 1, 0.14},
	{"Tide", 225, 1, 0.12},
	{"Aurora", 185, 1, 0.10},
	{"Dune", 80, 0.75, 0.11},
	{"Graphite", 265, 0.15, 0.02},
}

// Appearances of Settings: the system's, or always light or dark.
var Appearances = []string{"System", "Light", "Dark"}

func paletteNamed(name string) Palette {
	for _, p := range Palettes {
		if p.Name == name {
			return p
		}
	}
	return Palettes[0]
}

// Skin is a palette made into a theme, with the colors Settings draws
// beside the widgets'.
type Skin struct {
	*ui.Theme
	Sidebar ui.Color // the sidebar's tint, a step from the window's
	Card    ui.Color // cards, which lift off the window
	Shadow  ui.Color
	Divider ui.Color
}

func (p Palette) skin(dark bool) Skin {
	h, k := p.Hue, p.Tint
	o := func(l, c float32) ui.Color { return ui.Oklch(l, c, h) }
	if !dark {
		t := ui.LightTheme()
		t.Background = o(0.962, 0.022*k)
		t.Surface, t.SurfaceHover, t.SurfacePressed = o(0.945, 0.024*k), o(0.925, 0.028*k), o(0.9, 0.032*k)
		t.Border = o(0.875, 0.03*k)
		t.Text, t.TextMuted = o(0.25, 0.02*k), o(0.5, 0.03*k)
		t.Accent, t.AccentHover, t.AccentPressed = o(0.56, p.Accent), o(0.51, p.Accent), o(0.46, p.Accent)
		if p.Accent < 0.05 { // Graphite: ink, not color
			t.Accent, t.AccentHover, t.AccentPressed = o(0.3, 0.02), o(0.24, 0.02), o(0.18, 0.02)
		}
		t.Selection, t.Focus = t.Accent.Alpha(0.22), t.Accent.Alpha(0.5)
		t.Radius = 8
		return Skin{t, o(0.94, 0.03*k), o(0.995, 0.004*k), ui.RGBA(40, 30, 20, 0.07), o(0.91, 0.02*k)}
	}
	t := ui.DarkTheme()
	t.Background = o(0.2, 0.022*k)
	t.Surface, t.SurfaceHover, t.SurfacePressed = o(0.29, 0.016*k), o(0.32, 0.018*k), o(0.35, 0.02*k)
	t.Border = o(0.36, 0.02*k)
	t.Text, t.TextMuted = o(0.95, 0.01*k), o(0.72, 0.025*k)
	t.Accent, t.AccentHover, t.AccentPressed = o(0.64, p.Accent), o(0.69, p.Accent), o(0.58, p.Accent)
	if p.Accent < 0.05 {
		t.Accent, t.AccentHover, t.AccentPressed = o(0.86, 0.01), o(0.92, 0.01), o(0.8, 0.01)
		t.AccentText = o(0.2, 0.01)
	}
	t.Selection, t.Focus = t.Accent.Alpha(0.3), t.Accent.Alpha(0.55)
	t.Radius = 8
	return Skin{t, o(0.175, 0.024*k), o(0.245, 0.02*k), ui.RGBA(0, 0, 0, 0.35), o(0.3, 0.016*k)}
}

// applyAppearance sets the app's appearance, which the window's title bar
// and the dialogs follow too.
func applyAppearance(a string) {
	switch a {
	case Appearances[1]:
		mygo.Theme.SetSource(mygo.ThemeLight)
	case Appearances[2]:
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}
