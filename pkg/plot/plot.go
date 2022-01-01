package plot

import (
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

// Canvas describes the canvas on which we'll draw
type Canvas struct {
	Size  XY
	DPI   int
	Bleed XY
}

// Middle returns the middle point of the canvas
func (p Canvas) Middle() XY {
	return XY{(p.Size.X - 2*p.Bleed.X) / 2, (p.Size.Y - 2*p.Bleed.Y) / 2}
}

// Zero returns the zero point respecting the bleed
func (p Canvas) Zero() XY {
	return p.Bleed
}

// Up returns the upper right corner respecting the bleed
func (p Canvas) Up() XY {
	return p.Size.AddXY(p.Bleed.Mult(-1))
}

// Inner returns the plot size - 2*bleed
func (p Canvas) Inner() XY {
	return XY{p.Size.X - 2*p.Bleed.X, p.Size.Y - 2*p.Bleed.Y}
}

// Frame returns a frame around the canvas
func (p Canvas) Frame() []Drawable {
	const c = 2
	return []Drawable{
		Line{Start: XY{c, c}, End: XY{c, p.Size.Y - c}},
		Line{Start: XY{c, p.Size.Y - c}, End: XY{p.Size.X - c, p.Size.Y - c}},
		Line{Start: XY{p.Size.X - c, p.Size.Y - c}, End: XY{p.Size.X - c, c}},
		Line{Start: XY{p.Size.X - c, c}, End: XY{c, c}},
	}
}

// Frame returns a frame around the canvas respecting the bleed
func (p Canvas) FrameBleed() []Drawable {
	var (
		x = p.Bleed.X
		y = p.Bleed.Y
	)
	return []Drawable{
		Line{Start: XY{x, y}, End: XY{x, p.Size.Y - y}},
		Line{Start: XY{x, p.Size.Y - y}, End: XY{p.Size.X - x, p.Size.Y - y}},
		Line{Start: XY{p.Size.X - x, p.Size.Y - y}, End: XY{p.Size.X - x, y}},
		Line{Start: XY{p.Size.X - x, y}, End: XY{x, y}},
	}
}

// XY represents XY coordinates
type XY struct {
	X int
	Y int
}

// Add adds to the coordinates
func (xy XY) Add(x, y int) XY {
	return XY{xy.X + x, xy.Y + y}
}

// AddF adds floats which are cast to int beforehand
func (xy XY) AddF(x, y float64) XY {
	return XY{xy.X + int(x), xy.Y + int(y)}
}

// AddXY adds another XY pair
func (xy XY) AddXY(other XY) XY {
	return XY{xy.X + other.X, xy.Y + other.Y}
}

// Mult multiplies coordinates
func (xy XY) Mult(f int) XY {
	return XY{xy.X * f, xy.Y * f}
}

// Line is a drawable line
type Line struct {
	Start XY
	End   XY
}

func (Line) mustBeDrawable() {}

// BezierCurve is a drawable bezier curve
type BezierCurve struct {
	ControlPoints []XY
}

func (BezierCurve) mustBeDrawable() {}

// Debug helps debugging other drawables. Use `AsDebug()`.
type Debug struct {
	D Drawable
}

func (Debug) mustBeDrawable() {}

// Arc draws an arc
type Arc struct {
	P      XY
	Radius int
}

func (Arc) mustBeDrawable() {}

// AsDebug decorates drawables for debugging
func AsDebug(ds ...Drawable) []Drawable {
	res := make([]Drawable, 0, len(ds))
	for _, d := range ds {
		res = append(res, Debug{d})
	}
	return res
}

// Drawable marks all drawable elements
type Drawable interface {
	mustBeDrawable()
}

var _ Drawable = Line{}
var _ Drawable = BezierCurve{}

type Drawing []Drawable

// PlotFunc plots a drawing to some output device
type PlotFunc func(out io.Writer, p Canvas, d Drawing) error

// DrawFunc produces a new drawing
type DrawFunc func(p Canvas, args map[string]string) (d Drawing, err error)

// Run executes a drawing - use this as entry point for all "sketches"
func Run(p Canvas, d DrawFunc) {
	var (
		device = pflag.String("device", "png", "Output device. Only png is supported for now")
		output = pflag.StringP("output", "o", "", "path to the output file")
		args   = pflag.StringToString("args", nil, "args to pass to the drawing")
	)
	pflag.Parse()

	if *output == "" {
		log.Fatal("missing --output")
	}

	drawing, err := d(p, *args)
	if err != nil {
		log.WithError(err).Fatal("drawing failed")
	}

	var plot PlotFunc
	switch *device {
	case "png":
		plot = NewPNGPlotter()
	default:
		log.Fatalf("Unsupported device: %s", device)
	}

	f, err := os.OpenFile(*output, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		log.WithError(err).Fatal("cannot open output file")
	}
	defer f.Close()

	err = plot(f, p, drawing)
	if err != nil {
		log.WithError(err).Fatal("failed to plot drawing")
	}
}
