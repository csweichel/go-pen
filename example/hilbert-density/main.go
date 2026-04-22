package main

import (
	"math"
	"strconv"

	"github.com/aquilax/go-perlin"
	"github.com/csweichel/go-pen/pkg/plot"
)

func main() {
	plot.Run(plot.Canvas{
		Size:  plot.A4.Mult(4),
		Bleed: plot.XY{X: 15, Y: 40},
	}, func(p plot.Canvas, args map[string]string) (d plot.Drawing, err error) {
		order := parseIntArg(args, "order", 7)
		if order < 1 {
			order = 1
		}
		if order > 9 {
			order = 9
		}

		margin := 3
		inner := p.Inner().Add(-2*margin, -2*margin)
		origin := p.Bleed.Add(margin, margin)

		pts := hilbertPath(order)
		if len(pts) < 2 {
			return d, nil
		}

		gen := perlin.NewPerlin(2, 2, 3, 91991)
		side := (1 << order) - 1

		for i := 1; i < len(pts); i++ {
			a := gridToCanvas(pts[i-1], side, inner, origin)
			b := gridToCanvas(pts[i], side, inner, origin)
			base := plot.Line{Start: a, End: b}
			d = append(d, base)

			// Add offset companion strokes to increase local visual density.
			mid := plot.XY{
				X: (a.X + b.X) / 2,
				Y: (a.Y + b.Y) / 2,
			}
			xn := float64(mid.X-origin.X) / float64(inner.X)
			yn := float64(mid.Y-origin.Y) / float64(inner.Y)
			density := densityAt(gen, xn, yn)
			extra := int(math.Round(math.Pow(density, 1.6) * 10))

			offsets := layerOffsets(extra)
			for _, off := range offsets {
				d = append(d, offsetLine(base, off))
			}
		}

		if args["debug"] == "true" {
			d = append(d, plot.AsDebug(p.Frame()...)...)
			d = append(d, plot.AsDebug(p.FrameBleed()...)...)
		}
		return d, nil
	})
}

func parseIntArg(args map[string]string, key string, fallback int) int {
	raw, ok := args[key]
	if !ok || raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func densityAt(gen *perlin.Perlin, x, y float64) float64 {
	x = clamp01(x)
	y = clamp01(y)

	noise := 0.5 + 0.5*gen.Noise2D(x*2.5, y*2.5)
	stripes := 0.5 + 0.5*math.Sin(12*(x*0.7+y*1.1))
	r := math.Hypot(x-0.5, y-0.5) / 0.7071067811865476
	center := 1 - clamp01(r)

	v := 0.5*noise + 0.25*stripes + 0.25*center
	return clamp01(v)
}

func layerOffsets(extra int) []float64 {
	if extra <= 0 {
		return nil
	}

	// Build symmetric offsets around the base stroke:
	// +d, -d, +2d, -2d, ...
	step := 0.55
	res := make([]float64, 0, extra)
	for i := 0; i < extra; i++ {
		level := float64(i/2 + 1)
		off := level * step
		if i%2 == 1 {
			off = -off
		}
		res = append(res, off)
	}
	return res
}

func offsetLine(l plot.Line, dist float64) plot.Line {
	dx := float64(l.End.X - l.Start.X)
	dy := float64(l.End.Y - l.Start.Y)
	ll := math.Hypot(dx, dy)
	if ll == 0 {
		return l
	}
	nx := -dy / ll
	ny := dx / ll
	return plot.Line{
		Start: l.Start.AddF(nx*dist, ny*dist),
		End:   l.End.AddF(nx*dist, ny*dist),
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func gridToCanvas(p plot.XY, max int, inner, origin plot.XY) plot.XY {
	if max <= 0 {
		return origin
	}
	return plot.XY{
		X: origin.X + int(float64(p.X)*float64(inner.X)/float64(max)),
		Y: origin.Y + int(float64(p.Y)*float64(inner.Y)/float64(max)),
	}
}

func hilbertPath(order int) []plot.XY {
	n := 1 << order
	total := n * n
	res := make([]plot.XY, total)
	for d := 0; d < total; d++ {
		x, y := hilbertD2XY(n, d)
		res[d] = plot.XY{X: x, Y: y}
	}
	return res
}

func hilbertD2XY(n, d int) (x, y int) {
	t := d
	for s := 1; s < n; s *= 2 {
		rx := (t / 2) & 1
		ry := (t ^ rx) & 1
		x, y = hilbertRot(s, x, y, rx, ry)
		x += s * rx
		y += s * ry
		t /= 4
	}
	return x, y
}

func hilbertRot(n, x, y, rx, ry int) (int, int) {
	if ry == 0 {
		if rx == 1 {
			x = n - 1 - x
			y = n - 1 - y
		}
		x, y = y, x
	}
	return x, y
}
