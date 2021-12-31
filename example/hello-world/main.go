package main

import "github.com/csweichel/go-plot/pkg/plot"

func main() {
	plot.Run(plot.Plot{
		Size:  plot.A4.Mult(4),
		Bleed: plot.XY{X: 15, Y: 40},
	}, func(p plot.Plot, args map[string]string) (d plot.Drawing, err error) {
		for i := 0; i < 10; i++ {
			d = append(d, plot.Line{
				Start: p.Middle().Add(-10, -10),
				End:   p.Middle().Add(1100, i*50),
			})
		}

		d = append(d, plot.AsDebug(p.Frame()...)...)
		d = append(d, plot.AsDebug(p.FrameBleed()...)...)

		return d, nil
	})
}
