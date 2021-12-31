package main

import "github.com/csweichel/plotr/pkg/plotr"

func main() {
	plotr.Run(plotr.Plot{
		Size:  plotr.A4.Mult(4),
		Bleed: plotr.XY{X: 15, Y: 40},
	}, func(p plotr.Plot, args map[string]string) (d plotr.Drawing, err error) {
		for i := 0; i < 10; i++ {
			d = append(d, plotr.Line{
				Start: p.Middle().Add(-10, -10),
				End:   p.Middle().Add(1100, i*50),
			})
		}

		d = append(d, plotr.AsDebug(p.Frame()...)...)
		d = append(d, plotr.AsDebug(p.FrameBleed()...)...)

		return d, nil
	})
}
