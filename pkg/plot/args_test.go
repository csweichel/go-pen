package plot

import "testing"

func TestArgSpecNormalizeIntClamps(t *testing.T) {
	spec := IntArg("count", "line count", 2, 8, 4)

	got, err := spec.Normalize("99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "8" {
		t.Fatalf("unexpected normalized value: %q", got)
	}
}

func TestArgSpecNormalizeFloatClamps(t *testing.T) {
	spec := FloatArg("step", "spacing", 0.1, 1.5, 0.5)

	got, err := spec.Normalize("0.01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "0.1" {
		t.Fatalf("unexpected normalized value: %q", got)
	}
}

func TestArgSpecNormalizeBool(t *testing.T) {
	spec := BoolArg("enabled", "turn on feature", false)

	got, err := spec.Normalize("TRUE")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "true" {
		t.Fatalf("unexpected normalized value: %q", got)
	}
}

func TestArgSpecNormalizeUsesDefault(t *testing.T) {
	spec := FloatArg("step", "spacing", 0.1, 1.5, 0.5)

	got, err := spec.Normalize("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "0.5" {
		t.Fatalf("unexpected normalized value: %q", got)
	}
}
