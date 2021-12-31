package plotr

import (
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

type Plot struct {
	Size  XY
	DPI   int
	Bleed XY
}

func (p Plot) Middle() XY {
	return XY{(p.Size.X - 2*p.Bleed.X) / 2, (p.Size.Y - 2*p.Bleed.Y) / 2}
}

func (p Plot) Zero() XY {
	return p.Bleed
}

func (p Plot) Up() XY {
	return p.Size.AddXY(p.Bleed.Mult(-1))
}

func (p Plot) Frame() []Drawable {
	const c = 2
	return []Drawable{
		Line{Start: XY{c, c}, End: XY{c, p.Size.Y - c}},
		Line{Start: XY{c, p.Size.Y - c}, End: XY{p.Size.X - c, p.Size.Y - c}},
		Line{Start: XY{p.Size.X - c, p.Size.Y - c}, End: XY{p.Size.X - c, c}},
		Line{Start: XY{p.Size.X - c, c}, End: XY{c, c}},
	}
}

func (p Plot) FrameBleed() []Drawable {
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

type XY struct {
	X int
	Y int
}

func (xy XY) Add(x, y int) XY {
	return XY{xy.X + x, xy.Y + y}
}
func (xy XY) AddXY(other XY) XY {
	return XY{xy.X + other.X, xy.Y + other.Y}
}

func (xy XY) Mult(f int) XY {
	return XY{xy.X * f, xy.Y * f}
}

type Line struct {
	Start XY
	End   XY
}

type BezierCurve struct {
	ControlPoints []XY
}

type Debug struct {
	D Drawable
}

func AsDebug(ds ...Drawable) []Drawable {
	res := make([]Drawable, 0, len(ds))
	for _, d := range ds {
		res = append(res, Debug{d})
	}
	return res
}

type Drawable interface {
}

var _ Drawable = &Line{}
var _ Drawable = &BezierCurve{}

type Drawing []Drawable

type PlotFunc func(out io.Writer, p Plot, d Drawing) error
type DrawFunc func(p Plot, args map[string]string) (d Drawing, err error)

func Run(p Plot, d DrawFunc) {
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
