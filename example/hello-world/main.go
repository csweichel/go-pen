package main

import (
	"math"

	"github.com/csweichel/go-plot/pkg/plot"
)

func main() {
	plot.Run(plot.Plot{
		Size:  plot.A4.Mult(4),
		Bleed: plot.XY{X: 15, Y: 40},
	}, func(p plot.Plot, args map[string]string) (d plot.Drawing, err error) {
		const c = 50
		for i := 0; i < c; i++ {
			angle := (math.Pi / c) * float64(i)
			var amp = float64(p.Size.Y) * 1.7

			d = append(d, plot.Line{
				Start: p.Zero(),
				End:   p.Zero().Add(1100, int(amp+amp*math.Cos(angle))),
			})
			d = append(d, plot.Line{
				Start: p.Zero().Add(p.Inner().X, 0),
				End:   p.Zero().Add(-1100, int(amp+amp*math.Cos(angle))),
			})
		}

		d = append(d, plot.AsDebug(p.Frame()...)...)
		d = append(d, plot.AsDebug(p.FrameBleed()...)...)

		return d, nil
	})
}
