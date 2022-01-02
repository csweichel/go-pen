package plot

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type GCodeOpts struct {
	DrawFeed   int     `json:"drawFeed"`
	TravelFeed int     `json:"travelFeed"`
	TravelLift int     `json:"travelLift"`
	Scale      float64 `json:"scale"`
}

func NewDefaultGCodeOpts() *GCodeOpts {
	return &GCodeOpts{
		DrawFeed:   100,
		TravelFeed: 1000,
		TravelLift: 3,
		Scale:      0.1,
	}
}

func NewGCodePlotter(optFN string) (PlotFunc, error) {
	opts := NewDefaultGCodeOpts()
	if optFN != "" {
		f, err := os.Open(optFN)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		err = json.NewDecoder(f).Decode(opts)
		if err != nil {
			return nil, err
		}
	}

	var (
		up    GCodeCommand = GCodeLift{Height: opts.TravelLift, Feedrate: opts.TravelFeed}
		down  GCodeCommand = GCodeLift{Height: 0, Feedrate: opts.TravelFeed}
		state gcodeState
	)

	var draw func(w io.Writer, p Canvas, elem Drawable) (res []GCodeCommand, err error)
	draw = func(w io.Writer, p Canvas, elem Drawable) (res []GCodeCommand, err error) {
		switch e := elem.(type) {
		case Line:
			if state.Pos != e.Start {
				res = append(res,
					up,
					GCodeLinearMoveXY{
						P:        e.Start,
						Feedrate: opts.DrawFeed,
						Scale:    opts.Scale,
					},
					down,
				)
			}
			if state.Height != 0 {
				res = append(res, down)
			}
			res = append(res, GCodeLinearMoveXY{
				P:        e.End,
				Feedrate: opts.DrawFeed,
				Scale:    opts.Scale,
			})
		case Arc:
			err = fmt.Errorf("arc is not supported yet")
		case BezierCurve:
			err = fmt.Errorf("bezier is not supported yet")
		case Debug:
			// ignored
		default:
			err = fmt.Errorf("invalid drawing element: %v", e)
		}
		return
	}

	return func(out io.Writer, p Canvas, d Drawing) error {
		for _, e := range d {
			gcs, err := draw(out, p, e)
			if err != nil {
				return err
			}

			for _, cmd := range gcs {
				_, err := fmt.Fprintln(out, cmd.String())
				if err != nil {
					return err
				}

				cmd.ModifyState(&state)
			}
		}

		return nil
	}, nil
}

type gcodeState struct {
	Pos    XY
	Height int
}

type GCodeCommand interface {
	fmt.Stringer
	ModifyState(*gcodeState)
}

type GCodeLift struct {
	Height   int
	Feedrate int
}

func (l GCodeLift) String() string            { return fmt.Sprintf("G0 Z%d F%d", l.Height, l.Feedrate) }
func (l GCodeLift) ModifyState(s *gcodeState) { s.Height = l.Height }

type GCodeLinearMoveXY struct {
	P        XY
	Feedrate int
	Scale    float64
}

func (l GCodeLinearMoveXY) String() string {
	return fmt.Sprintf("G0 X%.3f Y%.3f F%d", l.Scale*float64(l.P.X), l.Scale*float64(l.P.Y), l.Feedrate)
}
func (l GCodeLinearMoveXY) ModifyState(s *gcodeState) { s.Pos = l.P }
