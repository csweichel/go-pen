package field

import (
	"math"

	"github.com/csweichel/go-pen/pkg/plot"

	"github.com/aquilax/go-perlin"
)

// NewVectorField produces a new vector field from vectors
func NewVectorField(counts, spacing plot.XY, sampler func(p plot.XY) Vector) *VectorField {
	res := make([][]Vector, counts.X)
	for x := 0; x < counts.X; x++ {
		res[x] = make([]Vector, counts.Y)
		for y := 0; y < counts.Y; y++ {
			res[x][y] = sampler(plot.XY{X: x * spacing.X, Y: y * spacing.Y})
		}
	}
	return &VectorField{
		Points:  res,
		Spacing: spacing,
	}
}

// VectorField represents a vector field
type VectorField struct {
	Points  [][]Vector
	Spacing plot.XY
}

// Finds the nearest point in the field. If the point is outside the field
// this function will return nil.
func (f *VectorField) Nearest(p plot.XY) *Vector {
	var (
		x = p.X / f.Spacing.X
		y = p.Y / f.Spacing.Y
	)
	if x < 0 || x >= len(f.Points) {
		return nil
	}

	col := f.Points[x]
	if y < 0 || y >= len(col) {
		return nil
	}

	return &col[y]
}

// Draw draws the field as a set of lines
func (field *VectorField) Draw(offset plot.XY) []plot.Drawable {
	res := make([]plot.Drawable, 0, len(field.Points))
	for x, xs := range field.Points {
		for y, p := range xs {
			s := plot.XY{X: x*field.Spacing.X + offset.X, Y: y*field.Spacing.Y + offset.Y}
			res = append(res, plot.Line{
				Start: s,
				End:   s.AddF(float64(p.Length)*math.Sin(p.Angle), float64(p.Length)*math.Cos(p.Angle)),
			})
		}
	}
	return res
}

// Vector represents a vector with a starting point, angle and length
type Vector struct {
	Angle  float64
	Length int
}

// NewPerlinNoiseField produces a new vector field spread across the canvas, where the angles are determined
// using Perlin noise. The counts determines the number of points in the vector field along X and Y.
// Alpha, beta, n and seed are perlin noise generator parameters.
func NewPerlinNoiseField(p plot.Canvas, counts plot.XY, alpha, beta float64, n int32, seed int64) *VectorField {
	var (
		inner   = p.Inner()
		dx      = float64(inner.X) / float64(counts.X-1)
		dy      = float64(inner.Y) / float64(counts.Y-1)
		spacing = plot.XY{X: int(dx), Y: int(dy)}
	)

	gen := perlin.NewPerlin(alpha, beta, n, seed)
	return NewVectorField(counts, spacing, func(pos plot.XY) Vector {
		ns := gen.Noise2D(float64(pos.X)/float64(inner.X), float64(pos.Y)/float64(inner.Y))
		angle := 2 * math.Pi * ns

		return Vector{
			Angle:  angle,
			Length: 35,
		}
	})
}

// TraceLine traces a line from the starting point following the angles of the nearest points until
// we've reached "steps" line count.s
func TraceLine(p plot.Canvas, field *VectorField, start plot.XY, len, steps int) (r []plot.Line) {
	if steps <= 0 {
		return
	}
	nearest := field.Nearest(start)
	if nearest == nil {
		return
	}

	angle := nearest.Angle
	end := start.Add(int(float64(len)*math.Cos(angle)), int(float64(len)*math.Sin(angle)))
	r = append(r, TraceLine(p, field, end, len, steps-1)...)
	r = append(r, plot.Line{
		Start: start.AddXY(p.Bleed),
		End:   end.AddXY(p.Bleed),
	})
	return
}
