package main

import (
	"math"
	"strconv"

	"github.com/csweichel/go-pen/pkg/contour"
	"github.com/csweichel/go-pen/pkg/plot"
)

type normPoint struct {
	X float64
	Y float64
}

type normRect struct {
	X float64
	Y float64
	W float64
	H float64
}

type template struct {
	Controls  []normPoint
	LineScale float64
}

type motif struct {
	Box      normRect
	Template string
}

type rect struct {
	Origin plot.XY
	Size   plot.XY
}

func main() {
	a3Portrait := plot.XY{X: 297, Y: 420}

	plot.Run(plot.Canvas{
		Size:     a3Portrait.Mult(4),
		Bleed:    plot.XY{X: 24, Y: 24},
		PenWidth: 1,
	}, func(p plot.Canvas, args map[string]string) (d plot.Drawing, err error) {
		variant := parseStringArg(args, "variant", "poster")
		baseLines := clamp(parseIntArg(args, "lines", 14), 3, 64)
		spacing := clampFloat(parseFloatArg(args, "spacing", 7.5), 2, 24)
		samples := clamp(parseIntArg(args, "samples", 36), 4, 160)
		caps := clamp(parseIntArg(args, "caps", 18), 4, 80)
		tension := clamp01(parseFloatArg(args, "tension", 0.18))

		opts := contour.Opts{
			Lines:          baseLines,
			Spacing:        spacing,
			SamplesPerSpan: samples,
			CapSegments:    caps,
			Tension:        tension,
		}

		switch variant {
		case "poster":
			d, err = posterDrawing(p, opts)
		default:
			d, err = singleDrawing(p, variant, opts)
		}
		if err != nil {
			return nil, err
		}

		if args["debug"] == "true" {
			d = append(d, plot.AsDebug(p.FrameBleed()...)...)
		}
		return d, nil
	}, plot.WithArgSchema(
		plot.IntArg("lines", "Number of contour lanes", 3, 64, 14),
		plot.FloatArg("spacing", "Distance between contour lanes", 2, 24, 7.5),
		plot.IntArg("samples", "Spline samples per centerline span", 4, 160, 36),
		plot.IntArg("caps", "Rounded end-cap resolution", 4, 80, 18),
		plot.FloatArg("tension", "Centerline spline tension", 0, 1, 0.18),
	))
}

func posterDrawing(p plot.Canvas, base contour.Opts) (plot.Drawing, error) {
	templates := contourTemplates()
	layout := []motif{
		{
			Box:      normRect{X: 0.02, Y: 0.03, W: 0.45, H: 0.18},
			Template: "ribbon",
		},
		{
			Box:      normRect{X: 0.56, Y: 0.03, W: 0.37, H: 0.33},
			Template: "comb",
		},
		{
			Box:      normRect{X: 0.18, Y: 0.21, W: 0.32, H: 0.17},
			Template: "s-bend",
		},
		{
			Box:      normRect{X: 0.03, Y: 0.38, W: 0.24, H: 0.34},
			Template: "wave",
		},
		{
			Box:      normRect{X: 0.28, Y: 0.46, W: 0.30, H: 0.28},
			Template: "hook",
		},
		{
			Box:      normRect{X: 0.58, Y: 0.43, W: 0.35, H: 0.49},
			Template: "u",
		},
		{
			Box:      normRect{X: 0.05, Y: 0.78, W: 0.54, H: 0.15},
			Template: "base",
		},
	}

	d := make(plot.Drawing, 0, 4096)
	for _, m := range layout {
		tpl, ok := templates[m.Template]
		if !ok {
			continue
		}

		box := boxRect(p, m.Box)
		ctrl := fitPoints(box, tpl.Controls)

		opts := scaledOpts(base, box, tpl.LineScale)
		lines, err := contour.Continuous(ctrl, opts)
		if err != nil {
			return nil, err
		}
		d = append(d, plot.AsDrawable(lines...)...)
	}
	return d, nil
}

func singleDrawing(p plot.Canvas, variant string, base contour.Opts) (plot.Drawing, error) {
	tpl, ok := contourTemplates()[variant]
	if !ok {
		tpl = contourTemplates()["ribbon"]
	}

	box := boxRect(p, normRect{X: 0.08, Y: 0.08, W: 0.84, H: 0.84})
	ctrl := fitPoints(box, tpl.Controls)
	opts := scaledOpts(base, box, math.Max(1.15, tpl.LineScale*1.2))

	lines, err := contour.Continuous(ctrl, opts)
	if err != nil {
		return nil, err
	}
	return plot.AsDrawable(lines...), nil
}

func contourTemplates() map[string]template {
	return map[string]template{
		"ribbon": {
			LineScale: 1.05,
			Controls: []normPoint{
				{X: 0.14, Y: 0.86},
				{X: 0.14, Y: 0.18},
				{X: 0.40, Y: 0.18},
				{X: 0.42, Y: 0.70},
				{X: 0.58, Y: 0.70},
				{X: 0.74, Y: 0.42},
				{X: 0.90, Y: 0.80},
			},
		},
		"comb": {
			LineScale: 1.2,
			Controls: []normPoint{
				{X: 0.20, Y: 0.88},
				{X: 0.20, Y: 0.18},
				{X: 0.84, Y: 0.18},
				{X: 0.84, Y: 0.36},
				{X: 0.32, Y: 0.36},
				{X: 0.32, Y: 0.54},
				{X: 0.84, Y: 0.54},
				{X: 0.84, Y: 0.72},
				{X: 0.28, Y: 0.72},
			},
		},
		"s-bend": {
			LineScale: 0.9,
			Controls: []normPoint{
				{X: 0.14, Y: 0.78},
				{X: 0.32, Y: 0.78},
				{X: 0.50, Y: 0.22},
				{X: 0.66, Y: 0.78},
				{X: 0.86, Y: 0.22},
			},
		},
		"wave": {
			LineScale: 0.95,
			Controls: []normPoint{
				{X: 0.58, Y: 0.92},
				{X: 0.18, Y: 0.76},
				{X: 0.72, Y: 0.56},
				{X: 0.26, Y: 0.36},
				{X: 0.74, Y: 0.14},
			},
		},
		"hook": {
			LineScale: 1.0,
			Controls: []normPoint{
				{X: 0.78, Y: 0.94},
				{X: 0.28, Y: 0.78},
				{X: 0.74, Y: 0.38},
				{X: 0.40, Y: 0.12},
				{X: 0.90, Y: 0.08},
			},
		},
		"u": {
			LineScale: 1.25,
			Controls: []normPoint{
				{X: 0.18, Y: 0.12},
				{X: 0.18, Y: 0.88},
				{X: 0.82, Y: 0.88},
				{X: 0.82, Y: 0.12},
			},
		},
		"base": {
			LineScale: 1.35,
			Controls: []normPoint{
				{X: 0.88, Y: 0.24},
				{X: 0.16, Y: 0.24},
				{X: 0.16, Y: 0.78},
				{X: 0.82, Y: 0.78},
				{X: 0.82, Y: 0.44},
				{X: 0.28, Y: 0.44},
			},
		},
	}
}

func scaledOpts(base contour.Opts, box rect, scale float64) contour.Opts {
	opts := base
	maxLines := max(4, int(math.Floor(0.45*float64(min(box.Size.X, box.Size.Y))/base.Spacing)))
	opts.Lines = clamp(int(math.Round(float64(base.Lines)*scale)), 4, maxLines)
	return opts
}

func boxRect(p plot.Canvas, box normRect) rect {
	inner := p.Inner()
	origin := plot.XY{
		X: p.Bleed.X + int(math.Round(box.X*float64(inner.X))),
		Y: p.Bleed.Y + int(math.Round(box.Y*float64(inner.Y))),
	}
	size := plot.XY{
		X: int(math.Round(box.W * float64(inner.X))),
		Y: int(math.Round(box.H * float64(inner.Y))),
	}
	return rect{Origin: origin, Size: size}
}

func fitPoints(box rect, points []normPoint) []plot.XY {
	res := make([]plot.XY, len(points))
	for i, pt := range points {
		res[i] = plot.XY{
			X: box.Origin.X + int(math.Round(pt.X*float64(box.Size.X))),
			Y: box.Origin.Y + int(math.Round(pt.Y*float64(box.Size.Y))),
		}
	}
	return res
}

func parseStringArg(args map[string]string, key string, fallback string) string {
	raw, ok := args[key]
	if !ok || raw == "" {
		return fallback
	}
	return raw
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

func clampFloat(v, lo, hi float64) float64 {
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
