package main

import "github.com/egoist/mygo/ui"

// The glyphs of Settings' sections, drawn for 24×24 with 2-point strokes.
func glyph(body string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`))
}

var (
	glyphGeneral    = glyph(`<path d="M4 6h9M17 6h3M4 12h3M11 12h9M4 18h11M19 18h1"/><circle cx="15" cy="6" r="2"/><circle cx="9" cy="12" r="2"/><circle cx="17" cy="18" r="2"/>`)
	glyphAppearance = glyph(`<path d="M12 3a9 9 0 1 0 0 18c1.1 0 1.8-.8 1.8-1.8 0-.5-.2-.9-.5-1.2-.3-.3-.5-.7-.5-1.2 0-1 .8-1.8 1.8-1.8h2.2A4.2 4.2 0 0 0 21 10.8C21 6.5 17 3 12 3z"/><circle cx="7.5" cy="11.5" r="1"/><circle cx="10" cy="7.5" r="1"/><circle cx="15" cy="7.5" r="1"/>`)
	glyphMenu       = glyph(`<path d="M9 6h11M9 12h11M9 18h11"/><path d="M4.5 6h.01M4.5 12h.01M4.5 18h.01" stroke-width="3"/>`)
	glyphNewFile    = glyph(`<path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5M12 11v6M9 14h6"/>`)
	glyphOpenIn     = glyph(`<rect x="3" y="4" width="18" height="16" rx="3"/><path d="M7.5 9.5l3 2.5-3 2.5M13 15h3.5"/>`)
	glyphFolders    = glyph(`<path d="M3 7.5A2.5 2.5 0 0 1 5.5 5H9l2 2h7.5A2.5 2.5 0 0 1 21 9.5v7a2.5 2.5 0 0 1-2.5 2.5h-13A2.5 2.5 0 0 1 3 16.5z"/>`)
	glyphActions    = glyph(`<path d="M13 2.5L4.5 13.5H11l-1 8 8.5-11H12z"/>`)
	glyphShortcuts  = glyph(`<rect x="2.5" y="6" width="19" height="12" rx="2.5"/><path d="M8 14.5h8"/><path d="M6.5 10h.01M10 10h.01M14 10h.01M17.5 10h.01" stroke-width="2.6"/>`)
	glyphUp         = glyph(`<path d="M6 15l6-6 6 6"/>`)
	glyphDown       = glyph(`<path d="M6 9l6 6 6-6"/>`)
)

// panes of Settings, in the sidebar's order, each with the colors of its
// badge: a hue, a chroma and a lightness.
var panes = []struct {
	ID, Title string
	Glyph     *ui.SVG
	Hue, C, L float32
}{
	{"general", "General", glyphGeneral, 260, 0.02, 0.6},
	{"appearance", "Appearance", glyphAppearance, 350, 0.17, 0.64},
	{"menu", "Menu Items", glyphMenu, 255, 0.16, 0.58},
	{"newfile", "New File", glyphNewFile, 150, 0.15, 0.64},
	{"openin", "Open In", glyphOpenIn, 265, 0.03, 0.34},
	{"folders", "Folders", glyphFolders, 225, 0.13, 0.66},
	{"actions", "Custom Actions", glyphActions, 55, 0.17, 0.7},
	{"shortcuts", "Shortcuts", glyphShortcuts, 300, 0.14, 0.6},
}

// badge draws a section's icon: its glyph in white on a rounded square of
// its color, lit from above.
func badge(c *ui.Context, g *ui.SVG, hue, chroma, l, size float32) {
	top, bottom := ui.Oklch(min(l+0.1, 0.95), chroma*0.85, hue-8), ui.Oklch(l-0.04, chroma, hue+8)
	ui.Column(c).Size(size, size).Radius(size*0.27).Center().
		LinearGradient(ui.LinearGradient{From: top, To: bottom, Angle: 180, Oklab: true}).
		Shadow(0, 0.5, 1.5, 0, ui.RGBA(0, 0, 0, 0.18)).
		Children(func() {
			ui.Icon(c, g).FontSize(size * 0.6).TextColor(ui.Hex("#ffffff"))
		})
}
