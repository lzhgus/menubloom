package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// cursor is a mouse pointer, in a 32×32 box.
var cursor = [][2]float64{{9, 4}, {9, 25}, {14, 20.5}, {17.5, 28}, {21, 26.5}, {17.5, 19}, {24, 19}}

// inside reports whether (x, y) is inside the polygon, by the even-odd rule.
func inside(poly [][2]float64, x, y float64) bool {
	in := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		a, b := poly[i], poly[j]
		if (a[1] > y) != (b[1] > y) && x < (b[0]-a[0])*(y-a[1])/(b[1]-a[1])+a[0] {
			in = !in
		}
	}
	return in
}

// trayIcon draws the menu bar icon: a pointer with a click's ripple, black
// on transparent for macOS to tint, at 32×32 for a 16-point icon.
func trayIcon() []byte {
	const n, ss = 32, 4 // supersampled for smooth edges
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			hits := 0
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					px, py := float64(x)+(float64(sx)+0.5)/ss, float64(y)+(float64(sy)+0.5)/ss
					dx, dy := px-9, py-4
					d := dx*dx + dy*dy
					ripple := d > 7*7 && d < 9*9 && (px < 7 || py < 2.5) // the arc above and left of the tip
					if inside(cursor, px, py) || ripple {
						hits++
					}
				}
			}
			img.Set(x, y, color.NRGBA{A: uint8(255 * hits / (ss * ss))})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}
