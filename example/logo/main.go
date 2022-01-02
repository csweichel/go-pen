package main

import (
	"math/rand"

	"github.com/csweichel/go-pen/pkg/clip"
	"github.com/csweichel/go-pen/pkg/field"
	"github.com/csweichel/go-pen/pkg/plot"
)

func main() {
	plot.Run(plot.Canvas{
		Size: plot.XY{X: 300, Y: 300},
	}, func(p plot.Canvas, args map[string]string) (d plot.Drawing, err error) {
		counts := plot.XY{X: 40, Y: 80}

		f := field.NewPerlinNoiseField(p, counts, 2.5, 1.2, 3, 1000)

		var fl []plot.Line
		rand.Seed(4589)
		for i := 0; i < 1000; i++ {
			s := plot.XY{X: rand.Intn(p.Size.X), Y: rand.Intn(p.Size.Y)}
			fl = append(fl, field.TraceLine(p, f, s, 10, 10)...)
		}

		c := 3
		mask := []plot.XY{
			p.Middle().Add(-10*c, -34*c),
			p.Middle().Add(-10*c, 24*c),
			p.Middle().Add(0, 33*c),
			p.Middle().Add(10*c, 24*c),
			p.Middle().Add(10*c, -34*c),
			p.Middle().Add(-10*c, -34*c),
		}
		nl := clip.ClipLines(fl, mask)
		for _, l := range nl {
			d = append(d, l.Offset(p.Bleed))
		}

		d = append(d, p.Frame()...)

		return d, nil
	})
}
