package main

import (
	"math"

	"github.com/csweichel/go-pen/pkg/field"
	"github.com/csweichel/go-pen/pkg/plot"
)

func main() {
	plot.Run(plot.Canvas{
		Size:  plot.A4.Mult(4),
		Bleed: plot.XY{X: 15, Y: 40},
	}, func(p plot.Canvas, args map[string]string) (d plot.Drawing, err error) {
		const (
			imagePath      = "../assets/sample.png"
			maxRadius      = 12
			minHatchSpacing = 2.5
			maxHatchSpacing = 8.0
		)

		counts := plot.XY{X: 40, Y: 55}
		imgField, err := field.NewImageField(imagePath, p, counts)
		if err != nil {
			return nil, err
		}

		debug := args["debug"] == "true"

		if debug {
			d = append(d, plot.AsDebug(imgField.Draw(p.Bleed)...)...)
		}

		inner := p.Inner()
		dx := float64(inner.X) / float64(counts.X)
		dy := float64(inner.Y) / float64(counts.Y)

		for x := 0; x < counts.X; x++ {
			for y := 0; y < counts.Y; y++ {
				sample := imgField.Nearest(plot.XY{
					X: int(float64(x) * dx),
					Y: int(float64(y) * dy),
				})
				if sample == nil {
					continue
				}

				darkness := 1.0 - sample.Luminosity
				if darkness < 0.05 {
					continue
				}

				cx := p.Bleed.X + int((float64(x)+0.5)*dx)
				cy := p.Bleed.Y + int((float64(y)+0.5)*dy)
				center := plot.XY{X: cx, Y: cy}

				radius := int(math.Round(darkness * maxRadius))
				if radius < 1 {
					radius = 1
				}

				// Draw circle outline
				d = append(d, plot.Arc{P: center, Radius: radius})

				// 45° hatching (low-left to upper-right) inside the circle.
				// Lines run along direction (1, 1)/√2. We sweep perpendicular
				// offsets `t` from the center. A line at offset `t` intersects
				// the circle x²+y²=r² where the perpendicular distance from
				// center equals |t|. The chord half-length is √(r²-t²).
				hatchSpacing := maxHatchSpacing - darkness*(maxHatchSpacing-minHatchSpacing)
				rf := float64(radius)
				inv := 1.0 / math.Sqrt(2)

				for t := -rf + hatchSpacing; t < rf; t += hatchSpacing {
					halfChord2 := rf*rf - t*t
					if halfChord2 <= 0 {
						continue
					}
					halfChord := math.Sqrt(halfChord2)

					// Perpendicular direction is (1, -1)/√2, line direction is (1, 1)/√2.
					// Center of chord in local coords: perpOffset * perpDir
					// Endpoints: chordCenter ± halfChord * lineDir
					px := t * inv  // perpendicular component x
					py := -t * inv // perpendicular component y

					d = append(d, plot.Line{
						Start: plot.XY{
							X: cx + int(px-halfChord*inv),
							Y: cy + int(py-halfChord*inv),
						},
						End: plot.XY{
							X: cx + int(px+halfChord*inv),
							Y: cy + int(py+halfChord*inv),
						},
					})
				}
			}
		}

		if debug {
			d = append(d, plot.AsDebug(p.Frame()...)...)
			d = append(d, plot.AsDebug(p.FrameBleed()...)...)
		}

		return d, nil
	})
}
