package tilepath

import (
	"testing"

	"github.com/csweichel/go-pen/pkg/plot"
)

func TestRotatePortClockwise(t *testing.T) {
	got := rotatePort(port{Edge: edgeNorth, Slot: 0}, 5, 1)
	want := port{Edge: edgeEast, Slot: 4}
	if got != want {
		t.Fatalf("unexpected rotated port: want %#v, got %#v", want, got)
	}

	got = rotatePort(port{Edge: edgeSouth, Slot: 1}, 5, 1)
	want = port{Edge: edgeWest, Slot: 3}
	if got != want {
		t.Fatalf("unexpected rotated port: want %#v, got %#v", want, got)
	}
}

func TestGenerateFindsSnakeLayout(t *testing.T) {
	grid, err := NewGrid(4, 4, snakeTiles(4, 4))
	if err != nil {
		t.Fatalf("unexpected grid error: %v", err)
	}

	result, err := Generate(plot.Canvas{
		Size:  plot.XY{X: 800, Y: 800},
		Bleed: plot.XY{X: 40, Y: 40},
	}, grid, Opts{
		Lanes:          3,
		ArcSegments:    10,
		SearchPasses:   10,
		SearchRestarts: 48,
		Seed:           7,
	})
	if err != nil {
		t.Fatalf("unexpected generate error: %v", err)
	}

	if len(result.Paths) != 3 {
		t.Fatalf("expected 3 traced paths, got %d", len(result.Paths))
	}

	wantComponent := grid.Rows * grid.Cols
	for i, size := range result.ComponentSizes {
		if size != wantComponent {
			t.Fatalf("component %d: want size %d, got %d", i, wantComponent, size)
		}
	}

	wantScore := 3 * wantComponent * wantComponent
	if result.Score != wantScore {
		t.Fatalf("unexpected score: want %d, got %d", wantScore, result.Score)
	}
}

func TestGenerateSeedChangesTiedLayout(t *testing.T) {
	grid, err := NewGrid(1, 1, []TileKind{TileLine})
	if err != nil {
		t.Fatalf("unexpected grid error: %v", err)
	}

	seen := map[int]struct{}{}
	for seed := 1; seed <= 32; seed++ {
		result, err := Generate(plot.Canvas{
			Size:  plot.XY{X: 200, Y: 200},
			Bleed: plot.XY{X: 20, Y: 20},
		}, grid, Opts{
			Lanes:          1,
			ArcSegments:    8,
			SearchPasses:   1,
			SearchRestarts: 1,
			Seed:           int64(seed),
		})
		if err != nil {
			t.Fatalf("seed %d: unexpected generate error: %v", seed, err)
		}

		seen[result.Rotations[0]] = struct{}{}
	}

	if len(seen) < 2 {
		t.Fatalf("expected multiple orientations across seeds, got %d", len(seen))
	}
}

func snakeTiles(rows int, cols int) []TileKind {
	tiles := make([]TileKind, rows*cols)
	for i := range tiles {
		tiles[i] = TileLine
	}

	for row := 0; row < rows-1; row++ {
		col := cols - 1
		if row%2 == 1 {
			col = 0
		}
		tiles[row*cols+col] = TileArc
		tiles[(row+1)*cols+col] = TileArc
	}

	return tiles
}
