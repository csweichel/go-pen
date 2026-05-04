package plot

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGCodeFlavor(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    GCodeFlavor
		wantErr bool
	}{
		{name: "default", input: "", want: GCodeFlavorVanilla},
		{name: "vanilla", input: "vanilla", want: GCodeFlavorVanilla},
		{name: "mk4s", input: "mk4s", want: GCodeFlavorMK4S},
		{name: "case insensitive", input: "MK4S", want: GCodeFlavorMK4S},
		{name: "invalid", input: "foo", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseGCodeFlavor(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected flavor: want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestNewGCodePlotterVanillaOutput(t *testing.T) {
	plotter, err := NewGCodePlotter("", "")
	if err != nil {
		t.Fatalf("unexpected error creating plotter: %v", err)
	}

	var buf bytes.Buffer
	err = plotter(&buf, Canvas{}, Drawing{
		Line{Start: XY{0, 0}, End: XY{10, 10}},
	})
	if err != nil {
		t.Fatalf("unexpected error plotting: %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "M862.3") {
		t.Fatalf("vanilla output unexpectedly contains mk4s prologue:\n%s", got)
	}

	want := "G0 X1.000 Y1.000 F1500\n"
	if got != want {
		t.Fatalf("unexpected vanilla output:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestNewGCodePlotterMK4SOutput(t *testing.T) {
	plotter, err := NewGCodePlotter("", "mk4s")
	if err != nil {
		t.Fatalf("unexpected error creating plotter: %v", err)
	}

	var buf bytes.Buffer
	err = plotter(&buf, Canvas{}, Drawing{
		Line{Start: XY{0, 0}, End: XY{10, 10}},
	})
	if err != nil {
		t.Fatalf("unexpected error plotting: %v", err)
	}

	want := strings.Join([]string{
		"; go-pen gcode flavor: mk4s",
		"; Prusa Buddy model check. Only applied when printing from a file.",
		`M862.3 P "MK4S"`,
		"; Use explicit modal setup so the printer does not inherit stale state.",
		"M17",
		"G21",
		"G90",
		"; Homing/start G-code is intentionally omitted for plotter rigs.",
		"G0 X1.000 Y1.000 F1500",
		"; Ensure all buffered motion finishes before the file ends.",
		"M400",
		"",
	}, "\n")

	got := buf.String()
	if got != want {
		t.Fatalf("unexpected mk4s output:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestNewGCodePlotterFlavorOverrideBeatsJSON(t *testing.T) {
	dir := t.TempDir()
	optsPath := filepath.Join(dir, "gcode-opts.json")
	err := os.WriteFile(optsPath, []byte(`{"flavor":"mk4s"}`), 0o644)
	if err != nil {
		t.Fatalf("unexpected error writing temp opts: %v", err)
	}

	plotter, err := NewGCodePlotter(optsPath, "vanilla")
	if err != nil {
		t.Fatalf("unexpected error creating plotter: %v", err)
	}

	var buf bytes.Buffer
	err = plotter(&buf, Canvas{}, Drawing{
		Line{Start: XY{0, 0}, End: XY{10, 10}},
	})
	if err != nil {
		t.Fatalf("unexpected error plotting: %v", err)
	}

	if strings.Contains(buf.String(), "M862.3") {
		t.Fatalf("flavor override did not win over json opts:\n%s", buf.String())
	}
}
