package clip

import (
	"github.com/csweichel/go-pen/pkg/plot"
	"github.com/ctessum/geom"
)

func InvertPolygon(bounds plot.XY, mask []plot.XY) []plot.XY {
	polyp := make(geom.Path, len(mask))
	for i, p := range mask {
		polyp[i] = geom.Point{X: float64(p.X), Y: float64(p.Y)}
	}
	poly := geom.Polygon{polyp}

	rect := geom.Polygon{
		geom.Path{
			geom.Point{X: 0, Y: 0},
			geom.Point{X: 0, Y: float64(bounds.Y)},
			geom.Point{X: float64(bounds.X), Y: float64(bounds.Y)},
			geom.Point{X: float64(bounds.X), Y: 0},
			geom.Point{X: 0, Y: 0},
		},
	}

	diff := rect.Difference(poly)
	res := make([]plot.XY, diff.Len())
	pts := diff.Points()
	for i := range res {
		p := pts()
		res[i] = plot.XY{X: int(p.X), Y: int(p.Y)}
	}
	return res
}

func ClipLines(lines []plot.Line, mask []plot.XY) []plot.Line {
	polyp := make(geom.Path, len(mask))
	for i, p := range mask {
		polyp[i] = geom.Point{X: float64(p.X), Y: float64(p.Y)}
	}
	poly := geom.Polygon{polyp}

	ls := make([]plot.Line, 0, len(lines))
	for _, l := range lines {
		line := geom.LineString{
			geom.Point{X: float64(l.Start.X), Y: float64(l.Start.Y)},
			geom.Point{X: float64(l.End.X), Y: float64(l.End.Y)},
		}
		res := line.Clip(poly)
		if res.Len() != 2 {
			continue
		}
		pts := res.Points()
		start := pts()
		end := pts()

		ls = append(ls, plot.Line{
			Start: plot.XY{X: int(start.X), Y: int(start.Y)},
			End:   plot.XY{X: int(end.X), Y: int(end.Y)},
		})
	}
	return ls
}
