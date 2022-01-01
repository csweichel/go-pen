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
			fl = append(fl, traceLine(p, f, s, 10, 10)...)
		}
		rand.Seed(189)
		for i := 0; i < 500; i++ {
			s := plot.XY{X: rand.Intn(p.Size.X), Y: rand.Intn(p.Size.Y)}
			fl = append(fl, traceLine(p, f, s, 10, 10)...)
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
