package curve

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"

	"github.com/csweichel/go-pen/pkg/plot"
	"github.com/sirupsen/logrus"
)

type Opts struct {
	Size   plot.XY
	Center plot.XY
}

// Continuous samples a continuous function and draws it within the bounding box
func Continuous(f func(x float64) float64, dt float64, opts Opts) ([]plot.Line, error) {
	if dt <= 0 {
		return nil, fmt.Errorf("dt must be positive")
	}
	if opts.Size.IsZero() {
		return nil, fmt.Errorf("size must not be zero")
	}

	var (
		start   = opts.Center.AddXY(opts.Size.Div(-2))
		current = start
		res     []plot.Line
	)
	for i := 0.0; i < 1.0; i += dt {
		var (
			x = int(i * float64(opts.Size.X))
			y = int(f(i) * float64(opts.Size.Y))
			p = start.Add(x, y)
		)
		res = append(res, plot.Line{Start: current, End: p})
		current = p
	}
	return res, nil
}

// Discrete draws a discrete curve represented by its points
func Discrete(xs, ys []float64, opts Opts) ([]plot.Line, error) {
	if opts.Size.IsZero() {
		return nil, fmt.Errorf("size must not be zero")
	}
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("len(x) must be equal to len(y)")
	}

	var (
		xmin = math.NaN()
		ymin = math.NaN()
		xmax = math.NaN()
		ymax = math.NaN()
	)
	for i := range xs {
		x, y := xs[i], ys[i]
		if x > xmax || math.IsNaN(xmax) {
			xmax = x
		}
		if y > ymax || math.IsNaN(ymax) {
			ymax = y
		}
		if x < xmin || math.IsNaN(xmin) {
			xmin = x
		}
		if y < ymin || math.IsNaN(ymin) {
			ymin = y
		}
	}
	var (
		dx = 1.0 / (xmax - xmin)
		dy = 1.0 / (ymax - ymin)
	)
	if math.IsNaN(dx) || math.IsInf(dx, 0) {
		return nil, fmt.Errorf("x range is zero")
	}
	if math.IsNaN(dy) || math.IsInf(dy, 0) {
		return nil, fmt.Errorf("y range is zero")
	}

	var (
		start   = opts.Center.AddXY(opts.Size.Div(-2))
		current = start
		res     []plot.Line
	)
	for i := range xs {
		var (
			x = (xs[i] - xs[0]) * dx * float64(opts.Size.X)
			y = (ys[i] - ys[0]) * dy * float64(opts.Size.Y)
			p = start.AddF(x, y)
		)
		res = append(res, plot.Line{Start: current, End: p})
		current = p
	}
	return res, nil
}

// ReadCSV reads a CSV file and parses the first and second column as X, resp. Y coordinates.
// If a row cannot be parsed, it is skipped.
func ReadCSV(fn string) (xs, ys []float64, err error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	fc := csv.NewReader(f)
	for {
		record, err := fc.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		if len(record) < 2 {
			continue
		}
		x, err := strconv.ParseFloat(record[0], 64)
		if err != nil {
			logrus.WithError(err).WithField("row", len(xs)).Warn("canot parse X, skipping row")
			continue
		}
		y, err := strconv.ParseFloat(record[1], 64)
		if err != nil {
			logrus.WithError(err).WithField("row", len(ys)).Warn("canot parse Y, skipping row")
			continue
		}
		xs = append(xs, x)
		ys = append(ys, y)
	}
	return
}
