package main

import (
	"math"
	"math/rand"

	"github.com/csweichel/go-plot/pkg/field"
	"github.com/csweichel/go-plot/pkg/plot"
)

func main() {
	plot.Run(plot.Canvas{
		Size:  plot.A4.Mult(4),
		Bleed: plot.XY{X: 15, Y: 40},
	}, func(p plot.Canvas, args map[string]string) (d plot.Drawing, err error) {
		counts := plot.XY{X: 40, Y: 80}

		f := field.NewPerlinNoiseField(p, counts, 2, 2, 3, 1000)
		d = append(d, plot.AsDebug(f.Draw(p.Bleed)...)...)

		rand.Seed(4589)
		for i := 0; i < 500; i++ {
			s := plot.XY{X: rand.Intn(p.Size.X), Y: rand.Intn(p.Size.Y)}
			d = append(d, traceLine(f, s, 10, 100)...)
		}
		rand.Seed(189)
		for i := 0; i < 500; i++ {
			s := plot.XY{X: rand.Intn(p.Size.X), Y: rand.Intn(p.Size.Y)}
			d = append(d, traceLine(f, s, 10, 100)...)
		}

		d = append(d, plot.AsDebug(p.Frame()...)...)
		d = append(d, plot.AsDebug(p.FrameBleed()...)...)

		return d, nil
	})
}

func traceLine(field *field.VectorField, start plot.XY, len, steps int) (r []plot.Drawable) {
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
		Start: start,
		End:   end,
	})
	r = append(r, traceLine(field, end, len, steps-1)...)
	return
}
