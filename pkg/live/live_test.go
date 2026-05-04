package live

import (
	"reflect"
	"testing"
)

func TestSplitManagedRenderArgs(t *testing.T) {
	args := []string{
		"--device", "json",
		"--device-opts=example/gcode-opts.json",
		"--output", "ignored.svg",
		"-o=ignored-again.svg",
		"--args", "density=5",
		"--optimise", "llo,custom",
		"--flag",
	}

	got := splitManagedRenderArgs(args)

	if got.Device != "json" {
		t.Fatalf("unexpected device: %q", got.Device)
	}
	if !got.DeviceExplicit {
		t.Fatal("expected device to be marked explicit")
	}
	if got.DeviceOpts != "example/gcode-opts.json" {
		t.Fatalf("unexpected device opts: %q", got.DeviceOpts)
	}
	if !got.DeviceOptsExplicit {
		t.Fatal("expected device opts to be marked explicit")
	}

	wantRemaining := []string{
		"--args", "density=5",
		"--optimise", "llo,custom",
		"--flag",
	}
	if !reflect.DeepEqual(got.RemainingArgs, wantRemaining) {
		t.Fatalf("unexpected remaining args:\nwant: %#v\ngot:  %#v", wantRemaining, got.RemainingArgs)
	}
}

func TestSketchNameForMainFile(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "empty", path: "", want: "sketch"},
		{name: "main file", path: "/tmp/hello-world/main.go", want: "hello-world"},
		{name: "named file", path: "/tmp/spiral.go", want: "spiral"},
		{name: "directory path", path: "/tmp/gallery/sketch", want: "sketch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sketchNameForMainFile(tt.path); got != tt.want {
				t.Fatalf("unexpected sketch name: want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestDownloadFilename(t *testing.T) {
	tests := []struct {
		name string
		base string
		ext  string
		want string
	}{
		{name: "simple", base: "hello-world", ext: "gcode", want: "hello-world.gcode"},
		{name: "spaces", base: "hello world", ext: ".gcode", want: "hello-world.gcode"},
		{name: "punctuation", base: "  weird:/name?.  ", ext: "gcode", want: "weird-name.gcode"},
		{name: "empty", base: "", ext: "gcode", want: "sketch.gcode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := downloadFilename(tt.base, tt.ext); got != tt.want {
				t.Fatalf("unexpected filename: want %q, got %q", tt.want, got)
			}
		})
	}
}
