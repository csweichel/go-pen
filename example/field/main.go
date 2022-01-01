package main

import (
	"github.com/csweichel/go-plot/pkg/field"
	"github.com/csweichel/go-plot/pkg/plot"
)

func main() {
	plot.Run(plot.Canvas{
		Size:  plot.A4.Mult(4),
		Bleed: plot.XY{X: 15, Y: 40},
	}, func(p plot.Canvas, args map[string]string) (d plot.Drawing, err error) {
		counts := plot.XY{X: 40, Y: 80}
		// spacing := plot.XY{X: p.Inner().X / (counts.X - 1), Y: p.Inner().Y / (counts.Y - 1)}

		// d = append(d, field.NewVectorField(counts, spacing, func(p plot.XY) field.Vector {
		// 	return field.Vector{Length: 10, Angle: 0.5}
		// }).Draw(p.Bleed)...)
		f := field.NewPerlinNoiseField(p, counts, 2, 2, 3, 1000)
		d = append(d, f.Draw(p.Bleed)...)

		d = append(d, plot.AsDebug(p.Frame()...)...)
		d = append(d, plot.AsDebug(p.FrameBleed()...)...)

		return d, nil
	})
}
