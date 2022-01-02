package plot

import (
	"fmt"
	"io"

	"github.com/fogleman/gg"
)

func NewPNGPlotter() PlotFunc {
	return func(out io.Writer, p Canvas, d Drawing) error {
		dc := gg.NewContext(p.Size.X, p.Size.Y)

		dc.Clear()
		dc.SetRGB(0, 0, 0)
		dc.SetLineWidth(1)

		for _, elem := range d {
			err := drawPNG(dc, p, elem)
			if err != nil {
				return err
			}
		}

		// enforce bleed
		dc.SetRGBA(1, 1, 1, 0.8)
		dc.DrawRectangle(0, 0, float64(p.Size.X), float64(p.Bleed.Y))
		dc.Fill()
		dc.DrawRectangle(0, float64(p.Size.Y-p.Bleed.Y), float64(p.Size.X), float64(p.Bleed.Y))
		dc.Fill()
		dc.DrawRectangle(0, 0, float64(p.Bleed.X), float64(p.Size.Y))
		dc.Fill()
		dc.DrawRectangle(float64(p.Size.X-p.Bleed.X), 0, float64(p.Bleed.X), float64(p.Size.Y))
		dc.Fill()

		return dc.EncodePNG(out)
	}
}

func drawPNG(dc *gg.Context, p Canvas, elem Drawable) error {
	switch e := elem.(type) {
	case Line:
		sx, sy := pngConvertXY(p, e.Start)
		ex, ey := pngConvertXY(p, e.End)
		dc.DrawLine(sx, sy, ex, ey)
		dc.Stroke()
	case Arc:
		dc.DrawArc(float64(e.P.X), float64(e.P.Y), float64(e.Radius), 0, 0)
		dc.Stroke()
	case BezierCurve:
		return fmt.Errorf("bezier is not supported yet")
	case Debug:
		dc.SetRGBA(0, 0.5, 0, 0.5)
		defer dc.SetRGBA(0, 0, 0, 1)

		return drawPNG(dc, p, e.D)
	default:
		return fmt.Errorf("invalid drawing element: %v", e)
	}
	return nil
}

func pngConvertXY(p Canvas, xy XY) (float64, float64) {
	return float64(xy.X), float64(p.Size.Y - xy.Y)
}
