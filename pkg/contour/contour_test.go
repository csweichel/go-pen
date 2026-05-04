package contour

import (
	"testing"

	"github.com/csweichel/go-pen/pkg/plot"
)

func TestContinuousProducesConnectedPath(t *testing.T) {
	lines, err := Continuous([]plot.XY{
		{X: 0, Y: 0},
		{X: 120, Y: 0},
	}, Opts{
		Lines:          4,
		Spacing:        12,
		SamplesPerSpan: 8,
		CapSegments:    10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("expected non-empty line set")
	}

	for i := 1; i < len(lines); i++ {
		if lines[i-1].End != lines[i].Start {
			t.Fatalf("path is not continuous between line %d and %d: %#v -> %#v", i-1, i, lines[i-1].End, lines[i].Start)
		}
	}
}

func TestContinuousAddsRoundedCaps(t *testing.T) {
	lines, err := Continuous([]plot.XY{
		{X: 0, Y: 0},
		{X: 100, Y: 0},
	}, Opts{
		Lines:          3,
		Spacing:        10,
		SamplesPerSpan: 6,
		CapSegments:    12,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	minX, maxX := lines[0].Start.X, lines[0].Start.X
	for _, line := range lines {
		for _, p := range []plot.XY{line.Start, line.End} {
			if p.X < minX {
				minX = p.X
			}
			if p.X > maxX {
				maxX = p.X
			}
		}
	}

	if minX >= 0 {
		t.Fatalf("expected rounded start cap to extend before x=0, got minX=%d", minX)
	}
	if maxX <= 100 {
		t.Fatalf("expected rounded end cap to extend past x=100, got maxX=%d", maxX)
	}
}

func TestContinuousRejectsInvalidInput(t *testing.T) {
	_, err := Continuous([]plot.XY{{X: 0, Y: 0}}, Opts{})
	if err == nil {
		t.Fatal("expected error for a single control point")
	}

	_, err = Continuous([]plot.XY{{X: 0, Y: 0}, {X: 1, Y: 1}}, Opts{Spacing: -1})
	if err == nil {
		t.Fatal("expected error for negative spacing")
	}

	_, err = Continuous([]plot.XY{{X: 0, Y: 0}, {X: 0, Y: 0}, {X: 0, Y: 0}}, Opts{})
	if err == nil {
		t.Fatal("expected error for duplicate-only control points")
	}
}
