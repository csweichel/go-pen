package field

import (
	"math"

	"github.com/csweichel/go-plot/pkg/plot"

	"github.com/aquilax/go-perlin"
)

// NewVectorField produces a new vector field from vectors
func NewVectorField(points []PointVector) *VectorField {
	return &VectorField{Points: points}
}

// VectorField represents a vector field
type VectorField struct {
	Points []PointVector
}

// Draw draws the field as a set of lines
func (field *VectorField) Draw() []plot.Drawable {
	res := make([]plot.Drawable, len(field.Points))
	for i, p := range field.Points {
		res[i] = plot.Line{
			Start: p.P,
			End:   p.P.AddF(float64(p.Length)*math.Sin(p.Angle), float64(p.Length)*math.Cos(p.Angle)),
		}
	}
	return res
}

// PointVector represents a vector with a starting point, angle and length
type PointVector struct {
	P      plot.XY
	Angle  float64
	Length int
}

// NewPerlinNoiseField produces a new vector field spread across the canvas, where the angles are determined
// using Perlin noise. The delta determines the number of points in the vector field along X and Y.
// Alpha, beta, n and seed are perlin noise generator parameters.
func NewPerlinNoiseField(p plot.Canvas, delta plot.XY, alpha, beta float64, n int32, seed int64) *VectorField {
	gen := perlin.NewPerlin(alpha, beta, n, seed)
	vec := make([]PointVector, 0, delta.X*delta.Y)
	var (
		inner = p.Inner()
		dx    = float64(inner.X) / float64(delta.X)
		dy    = float64(inner.Y) / float64(delta.Y)
	)
	for x := 0; x < delta.X; x++ {
		for y := 0; y < delta.Y; y++ {
			pos := plot.XY{X: int(dx*float64(x)) + p.Bleed.X, Y: int(dy*float64(y)) + p.Bleed.Y}
			ns := gen.Noise2D(float64(pos.X)/float64(inner.X), float64(pos.Y)/float64(inner.Y))
			angle := 2 * math.Pi * ns
			vec = append(vec, PointVector{
				P:      pos,
				Angle:  angle,
				Length: 10,
			})
		}
	}
	return NewVectorField(vec)
}
