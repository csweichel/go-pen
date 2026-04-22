package main

import (
	"math"
	"strconv"

	"github.com/csweichel/go-pen/pkg/plot"
)

func main() {
	// A3 in portrait orientation.
	a3 := plot.XY{X: 297, Y: 420}

	plot.Run(plot.Canvas{
		Size:     a3.Mult(4),
		Bleed:    plot.XY{X: 24, Y: 24},
		PenWidth: 1,
	}, func(p plot.Canvas, args map[string]string) (d plot.Drawing, err error) {
		loops := clamp(parseIntArg(args, "loops", 3), 2, 3)
		order := clamp(parseIntArg(args, "order", 6), 4, 7)
		maxExtra := clamp(parseIntArg(args, "thickness", 26), 8, 40)
		minExtra := clamp(parseIntArg(args, "base", 6), 0, maxExtra-1)

		path := hilbertPath(order)
		if len(path) < 2 {
			return d, nil
		}

		inner := p.Inner()
		margin := 32
		square := min(inner.X, inner.Y) - 2*margin
		if square <= 0 {
			return d, nil
		}

		center := p.Middle()
		maxGrid := (1 << order) - 1
		if maxGrid <= 0 {
			return d, nil
		}

		// Two or three nested Hilbert "loops".
		for li := 0; li < loops; li++ {
			scale := loopScale(li, loops)
			side := float64(square) * scale
			angle := loopRotation(li)

			for i := 1; i < len(path); i++ {
				a := mapHilbert(path[i-1], maxGrid, center, side, angle)
				b := mapHilbert(path[i], maxGrid, center, side, angle)
				base := plot.Line{Start: a, End: b}
				d = append(d, base)

				// Inside-out gradient: center is denser/thicker, edge is lighter/thinner.
				mid := plot.XY{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
				r := math.Hypot(float64(mid.X-center.X), float64(mid.Y-center.Y))
				rNorm := clamp01(r / (float64(square) * 0.52))
				inside := 1 - rNorm

				// Slightly boost inner loops so the center reads as a heavy core.
				loopBoost := 1.0 + 0.14*float64(li)
				extra := minExtra + int(math.Round(math.Pow(inside, 2.2)*float64(maxExtra-minExtra)*loopBoost))
				offsets := strokeOffsets(extra, 0.8)
				for _, off := range offsets {
					d = append(d, offsetLine(base, off))
				}
			}
		}

		if args["debug"] == "true" {
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

func loopScale(idx, loops int) float64 {
	if loops <= 1 {
		return 0.9
	}
	start := 0.98
	end := 0.56
	t := float64(idx) / float64(loops-1)
	return start + (end-start)*t
}

func loopRotation(idx int) float64 {
	return float64(idx) * 0.23
}

func mapHilbert(pt plot.XY, maxGrid int, center plot.XY, side float64, angle float64) plot.XY {
	ux := float64(pt.X)/float64(maxGrid)*2 - 1 // [-1,1]
	uy := float64(pt.Y)/float64(maxGrid)*2 - 1 // [-1,1]

	x := ux * side * 0.5
	y := uy * side * 0.5

	ca := math.Cos(angle)
	sa := math.Sin(angle)
	rx := x*ca - y*sa
	ry := x*sa + y*ca
	return center.AddF(rx, ry)
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

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
