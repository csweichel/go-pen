package plot

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type GCodeOpts struct {
	DrawFeed   int     `json:"drawFeed"`
	DrawLift   int     `json:"drawLift"`
	TravelFeed int     `json:"travelFeed"`
	TravelLift int     `json:"travelLift"`
	Scale      float64 `json:"scale"`
	Offset     XY      `json:"offset"`
}

func NewDefaultGCodeOpts() *GCodeOpts {
	return &GCodeOpts{
		DrawFeed:   1500,
		DrawLift:   0,
		TravelFeed: 4000,
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
		down  GCodeCommand = GCodeLift{Height: opts.DrawLift, Feedrate: opts.TravelFeed}
		state gcodeState
	)

	var draw func(w io.Writer, p Canvas, elem Drawable) (res []GCodeCommand, err error)
	draw = func(w io.Writer, p Canvas, elem Drawable) (res []GCodeCommand, err error) {
		switch e := elem.(type) {
		case Line:
			var (
				start = e.Start.AddXY(opts.Offset)
				end   = e.End.AddXY(opts.Offset)
			)
			if state.Pos != start {
				res = append(res,
					up,
					GCodeLinearMoveXY{
						P:        start,
						Feedrate: opts.DrawFeed,
						Scale:    opts.Scale,
					},
					down,
				)
			}
			if state.Height != opts.DrawLift {
				res = append(res, down)
			}
			res = append(res, GCodeLinearMoveXY{
				P:        end,
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
