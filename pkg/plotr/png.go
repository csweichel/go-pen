package plotr

import (
	"fmt"
	"io"

	"github.com/fogleman/gg"
)

func NewPNGPlotter() PlotFunc {
	return func(out io.Writer, p Plot, d Drawing) error {
		dc := gg.NewContext(p.Size.X, p.Size.Y)

		dc.Clear()
		dc.SetRGB(0, 0, 0)
		dc.SetLineWidth(1)

		for _, elem := range d {
			err := draw(dc, p, elem)
			if err != nil {
				return err
			}
		}

		return dc.EncodePNG(out)
	}
}

func draw(dc *gg.Context, p Plot, elem Drawable) error {
	switch e := elem.(type) {
	case Line:
		sx, sy := pngConvertXY(p, e.Start)
		ex, ey := pngConvertXY(p, e.End)
		dc.DrawLine(sx, sy, ex, ey)
		dc.Stroke()
	case BezierCurve:
		return fmt.Errorf("bezier is not supported yet")
	case Debug:
		dc.SetRGB(255, 0, 0)
		defer dc.SetRGB(0, 0, 0)
		return draw(dc, p, e.Drawable)
	default:
		return fmt.Errorf("invalid drawing element")
	}
	return nil
}

func pngConvertXY(p Plot, xy XY) (float64, float64) {
	return float64(xy.X), float64(p.Size.Y - xy.Y)
}