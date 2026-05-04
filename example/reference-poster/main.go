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

type rect struct {
	Origin plot.XY
	Size   plot.XY
}

type template struct {
	Controls     []normPoint
	LineScale    float64
	SpacingScale float64
	TensionScale float64
}

type motif struct {
	Name     string
	Box      normRect
	Template string
}

func main() {
	plot.Run(plot.Canvas{
		Size:     plot.XY{X: 300, Y: 450}.Mult(4),
		Bleed:    plot.XY{X: 24, Y: 24},
		PenWidth: 1,
	}, func(p plot.Canvas, args map[string]string) (plot.Drawing, error) {
		variant := parseStringArg(args, "variant", "poster")
		base := contour.Opts{
			Lines:          clamp(parseIntArg(args, "lines", 16), 4, 40),
			Spacing:        clampFloat(parseFloatArg(args, "spacing", 7.2), 3, 18),
			SamplesPerSpan: clamp(parseIntArg(args, "samples", 40), 8, 220),
			CapSegments:    clamp(parseIntArg(args, "caps", 20), 6, 80),
			Tension:        clamp01(parseFloatArg(args, "tension", 0.38)),
		}

		var (
			d   plot.Drawing
			err error
		)
		switch variant {
		case "poster":
			d, err = posterDrawing(p, base)
		default:
			d, err = singleDrawing(p, variant, base)
		}
		if err != nil {
			return nil, err
		}

		if args["debug"] == "true" {
			d = append(d, plot.AsDebug(p.FrameBleed()...)...)
		}
		return d, nil
	}, plot.WithArgSchema(
		plot.IntArg("lines", "Base number of contour lanes", 4, 40, 16),
		plot.FloatArg("spacing", "Base distance between contour lanes", 3, 18, 7.2),
		plot.IntArg("samples", "Samples per control span", 8, 220, 40),
		plot.IntArg("caps", "Rounded end-cap resolution", 6, 80, 20),
		plot.FloatArg("tension", "Base spline tension", 0, 1, 0.38),
	))
}

func posterDrawing(p plot.Canvas, base contour.Opts) (plot.Drawing, error) {
	templates := posterTemplates()
	layout := []motif{
		{
			Name:     "cream",
			Box:      normRect{X: 0.03, Y: 0.04, W: 0.92, H: 0.28},
			Template: "cream-spine",
		},
		{
			Name:     "rose",
			Box:      normRect{X: 0.04, Y: 0.15, W: 0.58, H: 0.29},
			Template: "rose-ribbon",
		},
		{
			Name:     "aqua",
			Box:      normRect{X: 0.02, Y: 0.42, W: 0.31, H: 0.32},
			Template: "aqua-snake",
		},
		{
			Name:     "orange",
			Box:      normRect{X: 0.26, Y: 0.49, W: 0.42, H: 0.38},
			Template: "orange-fold",
		},
		{
			Name:     "gold",
			Box:      normRect{X: 0.54, Y: 0.05, W: 0.41, H: 0.89},
			Template: "gold-spine",
		},
		{
			Name:     "base",
			Box:      normRect{X: 0.01, Y: 0.73, W: 0.63, H: 0.22},
			Template: "base-hook",
		},
	}

	d := make(plot.Drawing, 0, 8192)
	for _, m := range layout {
		tpl, ok := templates[m.Template]
		if !ok {
			continue
		}

		box := boxRect(p, m.Box)
		ctrl := fitPoints(box, tpl.Controls)
		opts := scaledOpts(base, box, tpl)
		lines, err := contour.Continuous(ctrl, opts)
		if err != nil {
			return nil, err
		}
		d = append(d, plot.AsDrawable(lines...)...)
	}
	return d, nil
}

func singleDrawing(p plot.Canvas, variant string, base contour.Opts) (plot.Drawing, error) {
	tpl, ok := posterTemplates()[variant]
	if !ok {
		tpl = posterTemplates()["rose-ribbon"]
	}

	box := boxRect(p, normRect{X: 0.08, Y: 0.08, W: 0.84, H: 0.84})
	ctrl := fitPoints(box, tpl.Controls)
	opts := scaledOpts(base, box, tpl)
	opts.Lines = clamp(int(math.Round(float64(opts.Lines)*1.1)), 6, 48)

	lines, err := contour.Continuous(ctrl, opts)
	if err != nil {
		return nil, err
	}
	return plot.AsDrawable(lines...), nil
}

func posterTemplates() map[string]template {
	return map[string]template{
		"cream-spine": {
			LineScale:    1.15,
			SpacingScale: 1.0,
			TensionScale: 1.25,
			Controls: []normPoint{
				{X: 0.10, Y: 0.93},
				{X: 0.10, Y: 0.14},
				{X: 0.28, Y: 0.14},
				{X: 0.30, Y: 0.70},
				{X: 0.45, Y: 0.68},
				{X: 0.59, Y: 0.46},
				{X: 0.72, Y: 0.20},
				{X: 0.92, Y: 0.20},
				{X: 0.92, Y: 0.74},
				{X: 0.74, Y: 0.74},
			},
		},
		"rose-ribbon": {
			LineScale:    1.02,
			SpacingScale: 0.98,
			TensionScale: 0.84,
			Controls: []normPoint{
				{X: 0.08, Y: 0.74},
				{X: 0.10, Y: 0.28},
				{X: 0.24, Y: 0.22},
				{X: 0.42, Y: 0.08},
				{X: 0.62, Y: 0.18},
				{X: 0.78, Y: 0.38},
				{X: 0.72, Y: 0.56},
				{X: 0.56, Y: 0.66},
				{X: 0.48, Y: 0.84},
				{X: 0.34, Y: 0.72},
				{X: 0.18, Y: 0.60},
				{X: 0.08, Y: 0.68},
			},
		},
		"aqua-snake": {
			LineScale:    1.0,
			SpacingScale: 1.0,
			TensionScale: 0.72,
			Controls: []normPoint{
				{X: 0.62, Y: 0.04},
				{X: 0.14, Y: 0.18},
				{X: 0.58, Y: 0.40},
				{X: 0.16, Y: 0.64},
				{X: 0.56, Y: 0.88},
				{X: 0.18, Y: 0.98},
			},
		},
		"orange-fold": {
			LineScale:    1.1,
			SpacingScale: 0.98,
			TensionScale: 0.95,
			Controls: []normPoint{
				{X: 0.80, Y: 0.08},
				{X: 0.22, Y: 0.08},
				{X: 0.20, Y: 0.22},
				{X: 0.34, Y: 0.36},
				{X: 0.70, Y: 0.30},
				{X: 0.86, Y: 0.54},
				{X: 0.34, Y: 0.74},
				{X: 0.94, Y: 0.92},
			},
		},
		"gold-spine": {
			LineScale:    1.12,
			SpacingScale: 0.98,
			TensionScale: 1.12,
			Controls: []normPoint{
				{X: 0.30, Y: 0.08},
				{X: 0.84, Y: 0.08},
				{X: 0.84, Y: 0.24},
				{X: 0.42, Y: 0.24},
				{X: 0.36, Y: 0.40},
				{X: 0.84, Y: 0.40},
				{X: 0.84, Y: 0.56},
				{X: 0.34, Y: 0.56},
				{X: 0.34, Y: 0.92},
				{X: 0.82, Y: 0.92},
				{X: 0.82, Y: 0.72},
			},
		},
		"base-hook": {
			LineScale:    1.08,
			SpacingScale: 1.02,
			TensionScale: 1.04,
			Controls: []normPoint{
				{X: 0.96, Y: 0.16},
				{X: 0.14, Y: 0.16},
				{X: 0.18, Y: 0.52},
				{X: 0.54, Y: 0.52},
				{X: 0.54, Y: 0.30},
				{X: 0.84, Y: 0.30},
				{X: 0.84, Y: 0.86},
				{X: 0.14, Y: 0.86},
			},
		},
	}
}

func scaledOpts(base contour.Opts, box rect, tpl template) contour.Opts {
	opts := base

	if tpl.SpacingScale > 0 {
		opts.Spacing = clampFloat(base.Spacing*tpl.SpacingScale, 2.5, 24)
	}
	if tpl.TensionScale > 0 {
		opts.Tension = clamp01(base.Tension * tpl.TensionScale)
	}

	scale := tpl.LineScale
	if scale <= 0 {
		scale = 1
	}

	maxLines := max(5, int(math.Floor(0.45*float64(min(box.Size.X, box.Size.Y))/opts.Spacing)))
	opts.Lines = clamp(int(math.Round(float64(base.Lines)*scale)), 5, maxLines)
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
	if raw, ok := args[key]; ok && raw != "" {
		return raw
	}
	return fallback
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
	return clampFloat(v, 0, 1)
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
