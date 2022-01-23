package main

import (
	"math"

	"github.com/csweichel/go-pen/pkg/curve"
	"github.com/csweichel/go-pen/pkg/plot"
)

func main() {
	plot.Run(plot.Canvas{
		Size:  plot.A4.Mult(4),
		Bleed: plot.XY{X: 15, Y: 40},
	}, func(p plot.Canvas, args map[string]string) (d plot.Drawing, err error) {
		cont, err := curve.Continuous(func(x float64) float64 {
			return (math.Sin(x*4*math.Pi) + 1) * 0.5
		}, 0.01, curve.Opts{
			Size:   p.Inner().Div(2),
			Center: p.Middle(),
		})
		if err != nil {
			return nil, err
		}
		d = append(d, plot.AsDrawable(cont...)...)

		xs, ys, err := curve.ReadCSV("example.csv")
		if err != nil {
			return nil, err
		}
		disc, err := curve.Discrete(
			xs, ys,
			curve.Opts{
				Size:   p.Inner().Div(2),
				Center: p.Middle(),
			},
		)
		if err != nil {
			return nil, err
		}
		d = append(d, plot.AsDrawable(disc...)...)

		d = append(d, plot.AsDebug(p.Frame()...)...)
		d = append(d, plot.AsDebug(p.FrameBleed()...)...)

		return d, nil
	})
}
