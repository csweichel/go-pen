package main

import (
	"math"
	"math/rand"

	"github.com/csweichel/go-plot/pkg/clip"
	"github.com/csweichel/go-plot/pkg/field"
	"github.com/csweichel/go-plot/pkg/plot"
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
			fl = append(fl, traceLine(p, f, s, 10, 10)...)
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

func traceLine(p plot.Canvas, field *field.VectorField, start plot.XY, len, steps int) (r []plot.Line) {
	if steps <= 0 {
		return
	}
	nearest := field.Nearest(start)
	if nearest == nil {
		return
	}

	angle := nearest.Angle
	end := start.Add(int(float64(len)*math.Cos(angle)), int(float64(len)*math.Sin(angle)))
	r = append(r, plot.Line{
		Start: start.AddXY(p.Bleed),
		End:   end.AddXY(p.Bleed),
	})
	r = append(r, traceLine(p, field, end, len, steps-1)...)
	return
}
