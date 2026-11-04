package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

// leaves are the sprout of the app icon as cubic Béziers in a 32×32 box,
// each drawn as a stroke that tapers to a point at both ends.
var leaves = [][4][2]float64{
	{{29.5, 6.5}, {22, 7.5}, {17, 12.5}, {15.8, 27}},
	{{2.5, 10}, {9.5, 9.5}, {15, 13}, {16.3, 20.5}},
}

func bezier(p [4][2]float64, t float64) (x, y float64) {
	u := 1 - t
	a, b, c, d := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
	return a*p[0][0] + b*p[1][0] + c*p[2][0] + d*p[3][0], a*p[0][1] + b*p[1][1] + c*p[2][1] + d*p[3][1]
}

// trayIcon draws the menu bar icon, black on transparent for macOS to tint,
// at 32×32 for a 16-point icon.
func trayIcon() []byte {
	const n, ss, steps, width = 32, 4, 160, 1.55 // supersampled for smooth edges
	type dot struct{ x, y, r float64 }
	var dots []dot
	for _, p := range leaves {
		for i := 0; i <= steps; i++ {
			t := float64(i) / steps
			x, y := bezier(p, t)
			dots = append(dots, dot{x, y, 0.25 + width*math.Pow(math.Sin(math.Pi*t), 0.7)})
		}
	}
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			hits := 0
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					px, py := float64(x)+(float64(sx)+0.5)/ss, float64(y)+(float64(sy)+0.5)/ss
					for _, d := range dots {
						if dx, dy := px-d.x, py-d.y; dx*dx+dy*dy < d.r*d.r {
							hits++
							break
						}
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
