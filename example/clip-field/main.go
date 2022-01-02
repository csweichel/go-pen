package main

import (
	"math/rand"

	"github.com/csweichel/go-pen/pkg/clip"
	"github.com/csweichel/go-pen/pkg/field"
	"github.com/csweichel/go-pen/pkg/plot"
)

func main() {
	plot.Run(plot.Canvas{
		Size:  plot.A4.Mult(4),
		Bleed: plot.XY{X: 15, Y: 40},
	}, func(p plot.Canvas, args map[string]string) (d plot.Drawing, err error) {
		counts := plot.XY{X: 40, Y: 80}

		f := field.NewPerlinNoiseField(p, counts, 2, 2, 3, 1000)
		d = append(d, plot.AsDebug(f.Draw(p.Bleed)...)...)

		var fl []plot.Line
		rand.Seed(4589)
		for i := 0; i < 5000; i++ {
			s := plot.XY{X: rand.Intn(p.Size.X), Y: rand.Intn(p.Size.Y)}
			fl = append(fl, field.TraceLine(p, f, s, 10, 10)...)
		}
		rand.Seed(189)
		for i := 0; i < 500; i++ {
			s := plot.XY{X: rand.Intn(p.Size.X), Y: rand.Intn(p.Size.Y)}
			fl = append(fl, field.TraceLine(p, f, s, 10, 10)...)
		}

		mask := clip.InvertPolygon(p.Inner(), []plot.XY{
			p.Middle().Add(200, -100),
			p.Middle().Add(100, 100),
			p.Middle().Add(-200, 100),
			p.Middle().Add(-100, -100),
		})
		nl := clip.ClipLines(fl, mask)
		for _, l := range nl {
			d = append(d, l.Offset(p.Bleed))
		}

		d = append(d, plot.AsDebug(p.Frame()...)...)
		d = append(d, plot.AsDebug(p.FrameBleed()...)...)

		return d, nil
	})
}
