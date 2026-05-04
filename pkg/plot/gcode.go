package plot

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type GCodeFlavor string

const (
	GCodeFlavorVanilla GCodeFlavor = "vanilla"
	GCodeFlavorMK4S    GCodeFlavor = "mk4s"
)

func ParseGCodeFlavor(v string) (GCodeFlavor, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", string(GCodeFlavorVanilla):
		return GCodeFlavorVanilla, nil
	case string(GCodeFlavorMK4S):
		return GCodeFlavorMK4S, nil
	default:
		return "", fmt.Errorf("unknown gcode flavor: %s", v)
	}
}

type GCodeOpts struct {
	DrawFeed   int     `json:"drawFeed"`
	DrawLift   int     `json:"drawLift"`
	TravelFeed int     `json:"travelFeed"`
	TravelLift int     `json:"travelLift"`
	Scale      float64 `json:"scale"`
	Offset     XY      `json:"offset"`
	Flavor     string  `json:"flavor,omitempty"`
}

func NewDefaultGCodeOpts() *GCodeOpts {
	return &GCodeOpts{
		DrawFeed:   1500,
		DrawLift:   0,
		TravelFeed: 4000,
		TravelLift: 3,
		Scale:      0.1,
		Flavor:     string(GCodeFlavorVanilla),
	}
}

func LoadGCodeOpts(optFN string, flavorOverride string) (*GCodeOpts, error) {
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

	flavor, err := ParseGCodeFlavor(opts.Flavor)
	if err != nil {
		return nil, err
	}
	if flavorOverride != "" {
		flavor, err = ParseGCodeFlavor(flavorOverride)
		if err != nil {
			return nil, err
		}
	}
	opts.Flavor = string(flavor)
	return opts, nil
}

func NewGCodePlotter(optFN string, flavorOverride string) (PlotFunc, error) {
	opts, err := LoadGCodeOpts(optFN, flavorOverride)
	if err != nil {
		return nil, err
	}
	return newNativeGCodePlotter(opts), nil
}

func newNativeGCodePlotter(opts *GCodeOpts) PlotFunc {
	var (
		up    GCodeCommand = GCodeLift{Height: opts.TravelLift, Feedrate: opts.TravelFeed}
		down  GCodeCommand = GCodeLift{Height: opts.DrawLift, Feedrate: opts.TravelFeed}
		state gcodeState
	)
	flavor, _ := ParseGCodeFlavor(opts.Flavor)

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
		err := writeGCodePrologue(out, flavor)
		if err != nil {
			return err
		}

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

		err = writeGCodeEpilogue(out, flavor)
		if err != nil {
			return err
		}

		return nil
	}
}

func writeGCodePrologue(w io.Writer, flavor GCodeFlavor) error {
	lines, err := gcodePrologueLines(flavor)
	if err != nil {
		return err
	}
	return writeGCodeLines(w, lines...)
}

func writeGCodeEpilogue(w io.Writer, flavor GCodeFlavor) error {
	lines, err := gcodeEpilogueLines(flavor)
	if err != nil {
		return err
	}
	return writeGCodeLines(w, lines...)
}

func gcodePrologueLines(flavor GCodeFlavor) ([]string, error) {
	switch flavor {
	case GCodeFlavorVanilla:
		return nil, nil
	case GCodeFlavorMK4S:
		return []string{
			"; go-pen gcode flavor: mk4s",
			`; Prusa Buddy model check. Only applied when printing from a file.`,
			`M862.3 P "MK4S"`,
			"; Use explicit modal setup so the printer does not inherit stale state.",
			"M17",
			"G21",
			"G90",
			"; Homing/start G-code is intentionally omitted for plotter rigs.",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported gcode flavor: %s", flavor)
	}
}

func gcodeEpilogueLines(flavor GCodeFlavor) ([]string, error) {
	switch flavor {
	case GCodeFlavorVanilla:
		return nil, nil
	case GCodeFlavorMK4S:
		return []string{
			"; Ensure all buffered motion finishes before the file ends.",
			"M400",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported gcode flavor: %s", flavor)
	}
}

func writeGCodeLines(w io.Writer, lines ...string) error {
	for _, line := range lines {
		_, err := fmt.Fprintln(w, line)
		if err != nil {
			return err
		}
	}
	return nil
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
