package main

import (
	"math"

	"github.com/csweichel/go-plot/pkg/field"
	"github.com/csweichel/go-plot/pkg/plot"
)

func main() {
	plot.Run(plot.Canvas{
		Size:  plot.A4.Mult(4),
		Bleed: plot.XY{X: 15, Y: 40},
	}, func(p plot.Canvas, args map[string]string) (d plot.Drawing, err error) {
		const (
			cx = 20
			cy = 40
		)

		var vs []field.Vector
		for x := 0; x < cx; x++ {
			for y := 0; y < cy; y++ {
				vs = append(vs, field.Vector{
					P:      p.Zero().Add((p.Inner().X/cx)*x, (p.Inner().Y/cy)*y),
					Length: 10,
					Angle:  math.Pi / cy * float64(y),
				})
			}
		}

		f := field.NewVectorField(vs)
		d = append(d, f.Draw()...)

		d = append(d, plot.AsDebug(p.Frame()...)...)
		d = append(d, plot.AsDebug(p.FrameBleed()...)...)

		return d, nil
	})
}
