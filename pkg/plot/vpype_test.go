package plot

import (
	"bytes"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestVpypeGCodeProfileConfigMK4S(t *testing.T) {
	opts := NewDefaultGCodeOpts()
	opts.Flavor = string(GCodeFlavorMK4S)

	cfg, err := vpypeGCodeProfileConfig(opts)
	if err != nil {
		t.Fatalf("unexpected error generating vpype gcode profile: %v", err)
	}

	for _, needle := range []string{
		"[gwrite.go_pen]",
		`unit = "px"`,
		"vertical_flip = true",
		`M862.3 P "MK4S"`,
		"segment_first = '''",
		`segment_last = "G0 X{x:.3f} Y{y:.3f} F1500\\n"`,
		"document_end = '''",
		"M400",
	} {
		if !strings.Contains(cfg, needle) {
			t.Fatalf("expected config to contain %q, got:\n%s", needle, cfg)
		}
	}
}

func TestVpypeGCodeArgsApplyTransformsAndPagesize(t *testing.T) {
	opts := NewDefaultGCodeOpts()
	opts.Scale = 0.25
	opts.Offset = XY{X: 10, Y: 20}

	got := vpypeGCodeArgs("cfg.toml", "input.svg", "output.gcode", Canvas{
		Size: XY{X: 840, Y: 1188},
	}, opts)

	want := []string{
		"-c", "cfg.toml",
		"read", "input.svg",
		"translate", "--", "10", "-20",
		"scale", "-o", "0", "0", "0.25", "0.25",
		"pagesize", "210x297",
		"linemerge",
		"linesimplify",
		"reloop",
		"linesort",
		"gwrite", "--profile", "go_pen", "output.gcode",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected vpype args:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestVpypeSVGGCodeArgsApplyTransforms(t *testing.T) {
	opts := NewDefaultGCodeOpts()
	opts.Scale = 0.25
	opts.Offset = XY{X: 10, Y: 20}

	got := vpypeSVGGCodeArgs("cfg.toml", "input.svg", "output.gcode", opts)

	want := []string{
		"-c", "cfg.toml",
		"read", "input.svg",
		"translate", "--", "10", "-20",
		"scale", "-o", "0", "0", "0.25", "0.25",
		"linemerge",
		"linesimplify",
		"reloop",
		"linesort",
		"gwrite", "--profile", "go_pen", "output.gcode",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected svg->gcode vpype args:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestPlotGCodeWithVpypeFallsBackWithoutGWrite(t *testing.T) {
	origLookPath := execLookPath
	origCommand := execCommand
	t.Cleanup(func() {
		execLookPath = origLookPath
		execCommand = origCommand
	})

	execLookPath = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "exit 2")
	}

	var buf bytes.Buffer
	err := PlotGCodeWithVpype(&buf, Canvas{}, Drawing{
		Line{Start: XY{0, 0}, End: XY{10, 10}},
	}, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "G0 X1.000 Y1.000 F1500\n"
	if buf.String() != want {
		t.Fatalf("unexpected fallback gcode:\nwant:\n%s\ngot:\n%s", want, buf.String())
	}
}
