package main

import (
	"math"

	"github.com/csweichel/go-pen/pkg/plot"
)

func main() {
	plot.Run(plot.Canvas{
		Size:  plot.A4.Mult(4),
		Bleed: plot.XY{X: 15, Y: 40},
	}, func(p plot.Canvas, args map[string]string) (d plot.Drawing, err error) {
		counts := plot.XY{X: 80, Y: 80}
		inner := p.Inner()
		dy := float64(inner.Y) / float64(counts.Y)
		dx := inner.X / counts.X

		for y := 0; y < counts.Y; y++ {
			prev := plot.XY{X: p.Bleed.X, Y: int(dy * float64(y))}
			for x := 0; x < counts.X+2; x++ {
				ns := math.Sin(4 * (float64(y)/float64(counts.Y) + float64(x)/float64(counts.X)) * math.Pi)

				end := plot.XY{X: x*dx + p.Bleed.X, Y: int(dy*(ns+float64(y))) + p.Bleed.Y}
				if x > 0 {
					d = append(d, plot.Line{
						Start: prev,
						End:   end,
					})
				}
				prev = end
			}
		}

		d = append(d, plot.AsDebug(p.Frame()...)...)
		d = append(d, plot.AsDebug(p.FrameBleed()...)...)

		return d, nil
	})
}
