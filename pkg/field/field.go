package field

import (
	"math"

	"github.com/csweichel/go-plot/pkg/plot"
)

// NewVectorField produces a new vector field from vectors
func NewVectorField(points []Vector) *VectorField {
	return &VectorField{Points: points}
}

// VectorField represents a vector field
type VectorField struct {
	Points []Vector
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

// Vector represents a vector with a starting point, angle and length
type Vector struct {
	P      plot.XY
	Angle  float64
	Length int
}
