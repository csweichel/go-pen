package plot

import (
	"fmt"
	"io"
	"strings"
)

// NewSVGPlotter returns a PlotFunc that writes SVG output.
func NewSVGPlotter() PlotFunc {
	return func(out io.Writer, p Canvas, d Drawing) error {
		w := p.Size.X
		h := p.Size.Y

		var b strings.Builder
		b.WriteString(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">
<rect width="100%%" height="100%%" fill="white"/>
<g stroke="black" stroke-width="%.3f" fill="none" stroke-linecap="round" stroke-linejoin="round">
`, w, h, w, h, p.StrokeWidth()))

		for _, elem := range d {
			if err := writeSVGElement(&b, p, elem); err != nil {
				return err
			}
		}

		b.WriteString("</g>\n</svg>\n")

		_, err := io.WriteString(out, b.String())
		return err
	}
}

func writeSVGElement(b *strings.Builder, p Canvas, elem Drawable) error {
	switch e := elem.(type) {
	case Line:
		sx, sy := svgConvertXY(p, e.Start)
		ex, ey := svgConvertXY(p, e.End)
		b.WriteString(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d"/>`+"\n", sx, sy, ex, ey))
	case Arc:
		cx, cy := svgConvertXY(p, e.P)
		b.WriteString(fmt.Sprintf(`<circle cx="%d" cy="%d" r="%d"/>`+"\n", cx, cy, e.Radius))
	case BezierCurve:
		if len(e.ControlPoints) < 2 {
			return nil
		}
		sx, sy := svgConvertXY(p, e.ControlPoints[0])
		b.WriteString(fmt.Sprintf(`<path d="M %d %d`, sx, sy))
		switch len(e.ControlPoints) {
		case 3:
			// Quadratic bezier
			c1x, c1y := svgConvertXY(p, e.ControlPoints[1])
			ex, ey := svgConvertXY(p, e.ControlPoints[2])
			b.WriteString(fmt.Sprintf(` Q %d %d %d %d`, c1x, c1y, ex, ey))
		case 4:
			// Cubic bezier
			c1x, c1y := svgConvertXY(p, e.ControlPoints[1])
			c2x, c2y := svgConvertXY(p, e.ControlPoints[2])
			ex, ey := svgConvertXY(p, e.ControlPoints[3])
			b.WriteString(fmt.Sprintf(` C %d %d %d %d %d %d`, c1x, c1y, c2x, c2y, ex, ey))
		default:
			// Approximate with line segments for higher-order curves
			for i := 1; i < len(e.ControlPoints); i++ {
				lx, ly := svgConvertXY(p, e.ControlPoints[i])
				b.WriteString(fmt.Sprintf(` L %d %d`, lx, ly))
			}
		}
		b.WriteString(`"/>` + "\n")
	case Debug:
		// Render debug elements in green with transparency, matching PNG behavior
		b.WriteString(`<g stroke="rgba(0,128,0,0.5)">` + "\n")
		if err := writeSVGElement(b, p, e.D); err != nil {
			return err
		}
		b.WriteString("</g>\n")
	default:
		return fmt.Errorf("invalid drawing element: %v", e)
	}
	return nil
}

// svgConvertXY flips Y to match the PNG plotter's coordinate system
// where Y=0 is at the bottom.
func svgConvertXY(p Canvas, xy XY) (int, int) {
	return xy.X, p.Size.Y - xy.Y
}
