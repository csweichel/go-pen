package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/csweichel/go-pen/pkg/plot"
	"github.com/csweichel/go-pen/pkg/tilepath"
)

type tileTool struct {
	ID          string
	Label       string
	Description string
	Short       string
	Kind        tilepath.TileKind
	Rotation    int
	Clear       bool
}

type tileCellOverride struct {
	Kind     string `json:"kind"`
	Rotation int    `json:"rotation"`
}

type tileEditorState struct {
	Cells map[string]tileCellOverride `json:"cells,omitempty"`
}

type tileLayout struct {
	Grid       tilepath.Grid
	Result     tilepath.Result
	State      tileEditorState
	Rows       int
	Cols       int
	TileSize   int
	Seed       int64
	Arc        int
	TargetTile int
}

var tileTools = []tileTool{
	{ID: "unset", Label: "unset", Description: "Remove the manual override for a cell", Short: "·", Clear: true},
	{ID: "line-h", Label: "line-h", Description: "Lock the cell to a horizontal line tile", Short: "L-", Kind: tilepath.TileLine, Rotation: 0},
	{ID: "line-v", Label: "line-v", Description: "Lock the cell to a vertical line tile", Short: "L|", Kind: tilepath.TileLine, Rotation: 1},
	{ID: "arc-sw", Label: "arc-sw", Description: "Lock the cell to an arc in the south-west corner", Short: "ASW", Kind: tilepath.TileArc, Rotation: 0},
	{ID: "arc-nw", Label: "arc-nw", Description: "Lock the cell to an arc in the north-west corner", Short: "ANW", Kind: tilepath.TileArc, Rotation: 1},
	{ID: "arc-ne", Label: "arc-ne", Description: "Lock the cell to an arc in the north-east corner", Short: "ANE", Kind: tilepath.TileArc, Rotation: 2},
	{ID: "arc-se", Label: "arc-se", Description: "Lock the cell to an arc in the south-east corner", Short: "ASE", Kind: tilepath.TileArc, Rotation: 3},
}

func main() {
	plot.Run(plot.Canvas{
		Size:     plot.A4.Mult(4),
		Bleed:    plot.XY{X: 20, Y: 20},
		PenWidth: 1,
	}, func(p plot.Canvas, args map[string]string) (plot.Drawing, error) {
		state, err := plot.DecodeInteractiveState[tileEditorState]()
		if err != nil {
			return nil, err
		}

		layout, err := buildTileLayout(p, args, state)
		if err != nil {
			return nil, err
		}

		d := layout.Result.Drawing()
		if args["debug"] == "true" {
			overlay, err := tilepath.DebugDrawing(p, layout.Grid, layout.Result)
			if err != nil {
				return nil, err
			}
			d = append(d, plot.AsDebug(p.FrameBleed()...)...)
			d = append(d, plot.AsDebug(overlay...)...)
		}
		return d, nil
	}, plot.WithArgSchema(
		plot.IntArg("tile", "Target tile size in plot units; may adjust slightly to cover more of the sheet", 20, 240, 100),
		plot.IntArg("arc", "Arc tile preference in percent; higher values place more quarter-arc tiles", 0, 100, 35),
		plot.IntArg("lanes", "Parallel lines or concentric arcs per tile", 1, 24, 9),
		plot.IntArg("segments", "Polyline segments per 90 degree arc", 4, 72, 18),
		plot.IntArg("passes", "Local search passes over the grid", 1, 24, 8),
		plot.IntArg("restarts", "Random-restart attempts for the orientation solver", 1, 120, 24),
		plot.IntArg("seed", "Deterministic search seed for the generated tile field and orientation search", 1, 9999, 17),
	), plot.WithInteractive(plot.InteractiveHooks{
		Describe: describeInteractive,
		Apply:    applyInteractive,
	}))
}

func describeInteractive(p plot.Canvas, args map[string]string, stateRaw json.RawMessage) (plot.InteractiveManifest, error) {
	state, err := decodeTileEditorState(stateRaw)
	if err != nil {
		return plot.InteractiveManifest{}, err
	}

	layout, err := buildTileLayout(p, args, state)
	if err != nil {
		return plot.InteractiveManifest{}, err
	}

	manifest := plot.InteractiveManifest{
		Canvas:      plot.InteractiveCanvas{Width: p.Size.X, Height: p.Size.Y},
		Tools:       interactiveTools(),
		Regions:     make([]plot.InteractiveRegion, 0, layout.Rows*layout.Cols),
		DefaultTool: "line-h",
	}

	for row := 0; row < layout.Rows; row++ {
		for col := 0; col < layout.Cols; col++ {
			idx := row*layout.Cols + col
			id := cellRegionID(row, col)
			override, locked := layout.State.Cells[id]
			currentKind := layout.Grid.At(row, col)
			currentRot := normalizeRotation(currentKind, layout.Result.Rotations[idx])
			currentTool := toolForKindRotation(currentKind, currentRot)

			x, y := cellRect(layout, p, row, col)
			region := plot.InteractiveRegion{
				ID:        id,
				X:         x,
				Y:         y,
				Width:     layout.TileSize,
				Height:    layout.TileSize,
				Stroke:    "rgba(125, 211, 252, 0.22)",
				Fill:      "rgba(125, 211, 252, 0.03)",
				TextColor: "#e5e7eb",
				Hint:      fmt.Sprintf("row %d, col %d | solver: %s", row+1, col+1, currentTool.Label),
			}

			if locked {
				lockedTool := toolForOverride(override)
				region.Label = lockedTool.Short
				region.Stroke = lockedStrokeColor(lockedTool.Kind)
				region.Fill = lockedFillColor(lockedTool.Kind)
				region.TextColor = lockedTextColor(lockedTool.Kind)
				region.Hint = fmt.Sprintf("row %d, col %d | locked: %s | solver: %s", row+1, col+1, lockedTool.Label, currentTool.Label)
			}

			manifest.Regions = append(manifest.Regions, region)
		}
	}

	return manifest, nil
}

func applyInteractive(_ plot.Canvas, _ map[string]string, stateRaw json.RawMessage, action plot.InteractiveAction) (json.RawMessage, error) {
	state, err := decodeTileEditorState(stateRaw)
	if err != nil {
		return nil, err
	}

	if state.Cells == nil {
		state.Cells = make(map[string]tileCellOverride)
	}

	tool, ok := findTileTool(action.Tool)
	if !ok {
		return nil, fmt.Errorf("unknown interactive tool %q", action.Tool)
	}
	if _, _, err := parseCellRegionID(action.Region); err != nil {
		return nil, err
	}

	if tool.Clear {
		delete(state.Cells, action.Region)
	} else {
		state.Cells[action.Region] = tileCellOverride{
			Kind:     tileKindName(tool.Kind),
			Rotation: normalizeRotation(tool.Kind, tool.Rotation),
		}
	}

	if len(state.Cells) == 0 {
		return nil, nil
	}
	data, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func buildTileLayout(p plot.Canvas, args map[string]string, state tileEditorState) (tileLayout, error) {
	targetTile := clamp(parseIntArg(args, "tile", 100), 20, min(p.Inner().X, p.Inner().Y))
	arcPreference := clamp(parseIntArg(args, "arc", 35), 0, 100)
	seed := int64(parseIntArg(args, "seed", 17))
	rows, cols, tileSize := chooseGrid(p.Inner(), targetTile)

	tiles := generatedTiles(rows, cols, seed, arcPreference)
	lockedRotations := make(map[int]int)
	for regionID, override := range state.Cells {
		row, col, err := parseCellRegionID(regionID)
		if err != nil {
			continue
		}
		if row < 0 || row >= rows || col < 0 || col >= cols {
			continue
		}
		kind, rot, ok := overrideSpec(override)
		if !ok {
			continue
		}
		idx := row*cols + col
		tiles[idx] = kind
		lockedRotations[idx] = rot
	}

	grid, err := tilepath.NewGrid(rows, cols, tiles)
	if err != nil {
		return tileLayout{}, err
	}

	result, err := tilepath.Generate(p, grid, tilepath.Opts{
		Lanes:           clamp(parseIntArg(args, "lanes", 9), 1, 24),
		ArcSegments:     clamp(parseIntArg(args, "segments", 18), 4, 72),
		SearchPasses:    clamp(parseIntArg(args, "passes", 8), 1, 24),
		SearchRestarts:  clamp(parseIntArg(args, "restarts", 24), 1, 120),
		Seed:            seed,
		TileSize:        tileSize,
		LockedRotations: lockedRotations,
	})
	if err != nil {
		return tileLayout{}, err
	}

	return tileLayout{
		Grid:       grid,
		Result:     result,
		State:      state,
		Rows:       rows,
		Cols:       cols,
		TileSize:   tileSize,
		Seed:       seed,
		Arc:        arcPreference,
		TargetTile: targetTile,
	}, nil
}

func generatedTiles(rows int, cols int, seed int64, arcPreference int) []tilepath.TileKind {
	tiles := make([]tilepath.TileKind, rows*cols)
	arcThreshold := float64(clamp(arcPreference, 0, 100)) / 100.0
	freqX := 1 + int(noise01(seed, 0, 0, 0x1234)*3)
	freqY := 1 + int(noise01(seed, 0, 0, 0x5678)*3)
	phaseX := noise01(seed, 0, 0, 0x9abc) * 2 * math.Pi
	phaseY := noise01(seed, 0, 0, 0xdef0) * 2 * math.Pi

	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			x := normalizedAxis(col, cols)
			y := normalizedAxis(row, rows)
			coarse := noise01(seed, row/2, col/2, 0xa5a5)
			medium := noise01(seed, row/3, col/3, 0x5a5a)
			fine := noise01(seed, row, col, 0x3141)
			wave := 0.5 + 0.5*math.Sin(float64(freqX)*2*math.Pi*x+phaseX)*math.Cos(float64(freqY)*2*math.Pi*y+phaseY)
			ring := 1 - distanceFromCenter(x, y)
			field := 0.35*coarse + 0.25*medium + 0.15*fine + 0.15*wave + 0.10*ring

			if field < arcThreshold {
				tiles[row*cols+col] = tilepath.TileArc
			} else {
				tiles[row*cols+col] = tilepath.TileLine
			}
		}
	}

	return tiles
}

func interactiveTools() []plot.InteractiveTool {
	tools := make([]plot.InteractiveTool, 0, len(tileTools))
	for _, tool := range tileTools {
		tools = append(tools, plot.InteractiveTool{
			ID:          tool.ID,
			Label:       tool.Label,
			Description: tool.Description,
		})
	}
	return tools
}

func findTileTool(id string) (tileTool, bool) {
	for _, tool := range tileTools {
		if tool.ID == id {
			return tool, true
		}
	}
	return tileTool{}, false
}

func toolForOverride(override tileCellOverride) tileTool {
	kind, rot, ok := overrideSpec(override)
	if !ok {
		return tileTools[0]
	}
	return toolForKindRotation(kind, rot)
}

func toolForKindRotation(kind tilepath.TileKind, rotation int) tileTool {
	rotation = normalizeRotation(kind, rotation)
	for _, tool := range tileTools {
		if tool.Clear {
			continue
		}
		if tool.Kind == kind && normalizeRotation(kind, tool.Rotation) == rotation {
			return tool
		}
	}
	return tileTools[0]
}

func overrideSpec(override tileCellOverride) (tilepath.TileKind, int, bool) {
	switch strings.ToLower(strings.TrimSpace(override.Kind)) {
	case "line":
		return tilepath.TileLine, normalizeRotation(tilepath.TileLine, override.Rotation), true
	case "arc":
		return tilepath.TileArc, normalizeRotation(tilepath.TileArc, override.Rotation), true
	default:
		return 0, 0, false
	}
}

func tileKindName(kind tilepath.TileKind) string {
	switch kind {
	case tilepath.TileArc:
		return "arc"
	default:
		return "line"
	}
}

func normalizeRotation(kind tilepath.TileKind, rotation int) int {
	switch kind {
	case tilepath.TileLine:
		rotation %= 2
		if rotation < 0 {
			rotation += 2
		}
		return rotation
	default:
		rotation %= 4
		if rotation < 0 {
			rotation += 4
		}
		return rotation
	}
}

func lockedStrokeColor(kind tilepath.TileKind) string {
	switch kind {
	case tilepath.TileArc:
		return "rgba(251, 146, 60, 0.92)"
	default:
		return "rgba(56, 189, 248, 0.92)"
	}
}

func lockedFillColor(kind tilepath.TileKind) string {
	switch kind {
	case tilepath.TileArc:
		return "rgba(251, 146, 60, 0.16)"
	default:
		return "rgba(56, 189, 248, 0.14)"
	}
}

func lockedTextColor(kind tilepath.TileKind) string {
	switch kind {
	case tilepath.TileArc:
		return "#ffedd5"
	default:
		return "#e0f2fe"
	}
}

func cellRegionID(row int, col int) string {
	return fmt.Sprintf("cell:%d:%d", row, col)
}

func parseCellRegionID(id string) (int, int, error) {
	var row int
	var col int
	if _, err := fmt.Sscanf(id, "cell:%d:%d", &row, &col); err != nil {
		return 0, 0, fmt.Errorf("invalid interactive region %q", id)
	}
	return row, col, nil
}

func cellRect(layout tileLayout, p plot.Canvas, row int, col int) (int, int) {
	plotX := layout.Result.Origin.X + col*layout.TileSize
	plotY := layout.Result.Origin.Y + (layout.Rows-1-row)*layout.TileSize
	return plotX, p.Size.Y - (plotY + layout.TileSize)
}

func decodeTileEditorState(raw json.RawMessage) (tileEditorState, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return tileEditorState{}, nil
	}

	var state tileEditorState
	if err := json.Unmarshal(raw, &state); err != nil {
		return tileEditorState{}, err
	}
	if state.Cells == nil {
		state.Cells = make(map[string]tileCellOverride)
	}
	return state, nil
}

func chooseGrid(inner plot.XY, targetTile int) (rows int, cols int, tileSize int) {
	targetTile = clamp(targetTile, 20, min(inner.X, inner.Y))
	targetMin := max(20, int(math.Round(float64(targetTile)*0.90)))
	targetMax := max(targetMin, int(math.Round(float64(targetTile)*1.10)))
	maxCols := max(1, inner.X/20)
	maxRows := max(1, inner.Y/20)

	bestBand := -1
	bestCoverage := -1
	bestDiff := 1 << 30
	bestRows := 1
	bestCols := 1
	bestSize := min(inner.X, inner.Y)

	for c := 1; c <= maxCols; c++ {
		for r := 1; r <= maxRows; r++ {
			size := min(inner.X/c, inner.Y/r)
			if size < 20 {
				continue
			}

			band := 0
			if size >= targetMin && size <= targetMax {
				band = 1
			}
			coverage := r * c * size * size
			diff := abs(size - targetTile)
			if band > bestBand || (band == bestBand && coverage > bestCoverage) || (band == bestBand && coverage == bestCoverage && diff < bestDiff) {
				bestBand = band
				bestCoverage = coverage
				bestDiff = diff
				bestRows = r
				bestCols = c
				bestSize = size
			}
		}
	}

	return bestRows, bestCols, bestSize
}

func parseIntArg(args map[string]string, key string, fallback int) int {
	raw, ok := args[key]
	if !ok || raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func clamp(v int, lo int, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func normalizedAxis(idx int, count int) float64 {
	if count <= 1 {
		return 0.5
	}
	return float64(idx) / float64(count-1)
}

func distanceFromCenter(x float64, y float64) float64 {
	dx := x - 0.5
	dy := y - 0.5
	d := math.Sqrt(dx*dx + dy*dy)
	maxD := math.Sqrt(0.5)
	if maxD == 0 {
		return 0
	}
	return clampFloat(d/maxD, 0, 1)
}

func clampFloat(v float64, lo float64, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func noise01(seed int64, row int, col int, salt uint64) float64 {
	x := uint64(seed)
	x += salt + 0x9e3779b97f4a7c15
	x += uint64(row) * 0xbf58476d1ce4e5b9
	x += uint64(col) * 0x94d049bb133111eb
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return float64(x>>11) / float64(1<<53)
}
