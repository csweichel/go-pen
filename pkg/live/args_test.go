package live

import (
	"reflect"
	"testing"

	"github.com/csweichel/go-pen/pkg/plot"
)

func TestParseSketchArgsValue(t *testing.T) {
	got, err := parseSketchArgsValue("variant=wave, enabled , spacing=0.75")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]string{
		"enabled": "true",
		"spacing": "0.75",
		"variant": "wave",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected parsed args:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestStripSketchArgsFlags(t *testing.T) {
	args := []string{
		"--flag",
		"--args", "variant=wave,lines=18",
		"--args=spacing=0.5",
	}

	remaining, sketchArgs := stripSketchArgsFlags(args)

	wantRemaining := []string{"--flag"}
	if !reflect.DeepEqual(remaining, wantRemaining) {
		t.Fatalf("unexpected remaining args:\nwant: %#v\ngot:  %#v", wantRemaining, remaining)
	}

	wantArgs := map[string]string{
		"lines":   "18",
		"spacing": "0.5",
		"variant": "wave",
	}
	if !reflect.DeepEqual(sketchArgs, wantArgs) {
		t.Fatalf("unexpected sketch args:\nwant: %#v\ngot:  %#v", wantArgs, sketchArgs)
	}
}

func TestBuildAuthoritativeArgsDropsDefaults(t *testing.T) {
	specs := []plot.ArgSpec{
		plot.IntArg("lines", "line count", 3, 64, 14),
		plot.FloatArg("spacing", "lane spacing", 2, 24, 7.5),
		plot.BoolArg("closed", "close the path", false),
	}

	got, err := buildAuthoritativeArgs(specs, map[string]string{
		"lines":   "20",
		"spacing": "7.5",
		"closed":  "false",
	}, "variant=wave")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]string{
		"lines":   "20",
		"variant": "wave",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected args:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestNewArgStateAppliesDefaultsAndRawExtras(t *testing.T) {
	specs := []plot.ArgSpec{
		plot.IntArg("lines", "line count", 3, 64, 14),
		plot.FloatArg("spacing", "lane spacing", 2, 24, 7.5),
	}

	state := newArgState(specs, map[string]string{
		"lines":   "20",
		"variant": "poster",
	})

	wantValues := map[string]string{
		"lines":   "20",
		"spacing": "7.5",
	}
	if !reflect.DeepEqual(state.Values, wantValues) {
		t.Fatalf("unexpected displayed values:\nwant: %#v\ngot:  %#v", wantValues, state.Values)
	}
	if state.Raw != "variant=poster" {
		t.Fatalf("unexpected raw args: %q", state.Raw)
	}
}
