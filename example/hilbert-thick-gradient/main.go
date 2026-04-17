package main

import (
	"math"
	"strconv"

	"github.com/csweichel/go-pen/pkg/plot"
)

func main() {
	// A3 landscape to emphasize width coverage.
	a3Landscape := plot.XY{X: 420, Y: 297}

	plot.Run(plot.Canvas{
		Size:  a3Landscape.Mult(4),
		Bleed: plot.XY{X: 24, Y: 24},
	}, func(p plot.Canvas, args map[string]string) (d plot.Drawing, err error) {
		order := clamp(parseIntArg(args, "order", 4), 2, 6)
		minExtra := clamp(parseIntArg(args, "min", 4), 0, 80)
		maxExtra := clamp(parseIntArg(args, "max", 40), minExtra+1, 120)
		step := parseFloatArg(args, "step", 0.72)
		if step <= 0 {
			step = 0.72
		}

		path := hilbertPath(order)
		if len(path) < 2 {
			return d, nil
		}

		inner := p.Inner()
		origin := p.Bleed
		gridMax := (1 << order) - 1
		if gridMax <= 0 {
			return d, nil
		}

		for i := 1; i < len(path); i++ {
			a := mapToRect(path[i-1], gridMax, origin, inner)
			b := mapToRect(path[i], gridMax, origin, inner)
			base := plot.Line{Start: a, End: b}
			d = append(d, base)

			mid := plot.XY{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
			g := thicknessGradient(mid, p)
			extra := minExtra + int(math.Round(g*float64(maxExtra-minExtra)))

			for _, off := range strokeOffsets(extra, step) {
				d = append(d, offsetLine(base, off))
			}
		}

		return d, nil
	})
}

// thicknessGradient produces a center-heavy inside-out gradient:
// thickest around the center, thinner towards the page edges.
func thicknessGradient(mid plot.XY, p plot.Canvas) float64 {
	inner := p.Inner()
	if inner.X <= 0 || inner.Y <= 0 {
		return 0
	}
	xn := float64(mid.X-p.Bleed.X) / float64(inner.X)
	yn := float64(mid.Y-p.Bleed.Y) / float64(inner.Y)
	xn = clamp01(xn)
	yn = clamp01(yn)

	dx := xn - 0.5
	dy := yn - 0.5
	r := math.Hypot(dx, dy) / 0.7071067811865476
	return clamp01(1 - r)
}

func mapToRect(pt plot.XY, maxGrid int, origin, size plot.XY) plot.XY {
	return plot.XY{
		X: origin.X + int(float64(pt.X)*float64(size.X)/float64(maxGrid)),
		Y: origin.Y + int(float64(pt.Y)*float64(size.Y)/float64(maxGrid)),
	}
}

func strokeOffsets(extra int, step float64) []float64 {
	if extra <= 0 {
		return nil
	}
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

func parseFloatArg(args map[string]string, key string, fallback float64) float64 {
	raw, ok := args[key]
	if !ok || raw == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return v
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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
