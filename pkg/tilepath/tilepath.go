package tilepath

import (
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/csweichel/go-pen/pkg/plot"
)

type TileKind int

const (
	TileArc TileKind = iota
	TileLine
)

type Grid struct {
	Rows  int
	Cols  int
	Tiles []TileKind
}

type Opts struct {
	Lanes          int
	ArcSegments    int
	SearchPasses   int
	SearchRestarts int
	Seed           int64
	TileSize       int
	LockedRotations map[int]int
}

type Result struct {
	Rotations      []int
	Paths          [][]plot.Line
	ComponentSizes []int
	Score          int
	CellSize       int
	Origin         plot.XY
}

func (r Result) Drawing() plot.Drawing {
	total := 0
	for _, path := range r.Paths {
		total += len(path)
	}

	d := make(plot.Drawing, 0, total)
	for _, path := range r.Paths {
		d = append(d, plot.AsDrawable(path...)...)
	}
	return d
}

func NewGrid(rows int, cols int, tiles []TileKind) (Grid, error) {
	if rows <= 0 || cols <= 0 {
		return Grid{}, fmt.Errorf("rows and cols must be positive")
	}
	if len(tiles) != rows*cols {
		return Grid{}, fmt.Errorf("need %d tiles, got %d", rows*cols, len(tiles))
	}

	res := make([]TileKind, len(tiles))
	copy(res, tiles)
	return Grid{
		Rows:  rows,
		Cols:  cols,
		Tiles: res,
	}, nil
}

func (g Grid) At(row int, col int) TileKind {
	return g.Tiles[g.index(row, col)]
}

func Generate(p plot.Canvas, g Grid, opts Opts) (Result, error) {
	opts, err := normalizeOpts(opts)
	if err != nil {
		return Result{}, err
	}
	if err := validateGrid(g); err != nil {
		return Result{}, err
	}
	if err := validateLockedRotations(g, opts); err != nil {
		return Result{}, err
	}

	cellSize, origin, err := fitGrid(p, g, opts)
	if err != nil {
		return Result{}, err
	}

	shapes := precomputeShapes(opts, cellSize)
	rotations, score, sizes := solve(g, opts, shapes)
	instances := buildInstances(g, opts, cellSize, origin, rotations, shapes)
	paths := tracePaths(instances)

	return Result{
		Rotations:      rotations,
		Paths:          quantizePaths(paths),
		ComponentSizes: sizes,
		Score:          score,
		CellSize:       cellSize,
		Origin:         origin,
	}, nil
}

func DebugDrawing(p plot.Canvas, g Grid, result Result) (plot.Drawing, error) {
	if err := validateGrid(g); err != nil {
		return nil, err
	}
	if len(result.Rotations) != g.Rows*g.Cols {
		return nil, fmt.Errorf("need %d rotations, got %d", g.Rows*g.Cols, len(result.Rotations))
	}

	cellSize := result.CellSize
	origin := result.Origin
	if cellSize <= 0 {
		var err error
		cellSize, origin, err = fitGrid(p, g, Opts{})
		if err != nil {
			return nil, err
		}
	}

	lines := debugGridLines(g, cellSize, origin)
	for row := 0; row < g.Rows; row++ {
		for col := 0; col < g.Cols; col++ {
			idx := g.index(row, col)
			offset := cellOrigin(origin, g.Rows, cellSize, row, col)
			lines = append(lines, debugTileGlyph(g.At(row, col), result.Rotations[idx], cellSize, offset)...)
		}
	}

	return plot.AsDrawable(lines...), nil
}

type edge int

const (
	edgeNorth edge = iota
	edgeEast
	edgeSouth
	edgeWest
)

type port struct {
	Edge edge
	Slot int
}

type point struct {
	X float64
	Y float64
}

type strand struct {
	Ports  [2]port
	Points []point
}

type shape struct {
	Strands    []strand
	PortLookup map[port]portRef
}

type portRef struct {
	Strand int
	End    int
}

type strandInstance struct {
	Points    []point
	Neighbors [2]neighborRef
}

type neighborRef struct {
	Strand int
	Port   int
	Valid  bool
}

func validateGrid(g Grid) error {
	if g.Rows <= 0 || g.Cols <= 0 {
		return fmt.Errorf("rows and cols must be positive")
	}
	if len(g.Tiles) != g.Rows*g.Cols {
		return fmt.Errorf("grid needs %d tiles, got %d", g.Rows*g.Cols, len(g.Tiles))
	}
	for i, kind := range g.Tiles {
		switch kind {
		case TileArc, TileLine:
		default:
			return fmt.Errorf("invalid tile kind at index %d: %d", i, kind)
		}
	}
	return nil
}

func validateLockedRotations(g Grid, opts Opts) error {
	for idx, rot := range opts.LockedRotations {
		if idx < 0 || idx >= len(g.Tiles) {
			return fmt.Errorf("locked rotation index %d out of range", idx)
		}
		allowed := allowedRotations(g.Tiles[idx])
		valid := false
		for _, candidate := range allowed {
			if rot == candidate {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("locked rotation %d is invalid for tile %d", rot, idx)
		}
	}
	return nil
}

func normalizeOpts(opts Opts) (Opts, error) {
	if opts.Lanes < 0 {
		return Opts{}, fmt.Errorf("lanes must be positive")
	}
	if opts.ArcSegments < 0 {
		return Opts{}, fmt.Errorf("arc segments must be positive")
	}
	if opts.SearchPasses < 0 {
		return Opts{}, fmt.Errorf("search passes must be positive")
	}
	if opts.SearchRestarts < 0 {
		return Opts{}, fmt.Errorf("search restarts must be positive")
	}
	if opts.TileSize < 0 {
		return Opts{}, fmt.Errorf("tile size must be positive")
	}

	if opts.Lanes == 0 {
		opts.Lanes = 8
	}
	if opts.ArcSegments == 0 {
		opts.ArcSegments = 12
	}
	if opts.SearchPasses == 0 {
		opts.SearchPasses = 8
	}
	if opts.SearchRestarts == 0 {
		opts.SearchRestarts = 24
	}
	if opts.Seed == 0 {
		opts.Seed = 1
	}
	return opts, nil
}

func fitGrid(p plot.Canvas, g Grid, opts Opts) (int, plot.XY, error) {
	inner := p.Inner()
	if opts.TileSize > 0 {
		cellSize := opts.TileSize
		gridW := cellSize * g.Cols
		gridH := cellSize * g.Rows
		if gridW > inner.X || gridH > inner.Y {
			return 0, plot.XY{}, fmt.Errorf("tile size %d does not fit a %dx%d grid into canvas inner area %dx%d", cellSize, g.Rows, g.Cols, inner.X, inner.Y)
		}
		return cellSize, p.Bleed, nil
	}

	cellSize := min(inner.X/g.Cols, inner.Y/g.Rows)
	if cellSize <= 0 {
		return 0, plot.XY{}, fmt.Errorf("canvas is too small for a %dx%d grid", g.Rows, g.Cols)
	}

	gridW := cellSize * g.Cols
	gridH := cellSize * g.Rows
	origin := plot.XY{
		X: p.Bleed.X + (inner.X-gridW)/2,
		Y: p.Bleed.Y + (inner.Y-gridH)/2,
	}
	return cellSize, origin, nil
}

func precomputeShapes(opts Opts, size int) map[TileKind][4]shape {
	res := make(map[TileKind][4]shape, 2)
	for _, kind := range []TileKind{TileArc, TileLine} {
		base := baseStrands(kind, opts.Lanes, size, opts.ArcSegments)
		var rotations [4]shape
		for rot := 0; rot < 4; rot++ {
			strands := make([]strand, len(base))
			lookup := make(map[port]portRef, len(base)*2)
			for i, s := range base {
				strands[i] = strand{
					Ports: [2]port{
						rotatePort(s.Ports[0], opts.Lanes, rot),
						rotatePort(s.Ports[1], opts.Lanes, rot),
					},
					Points: rotatePoints(s.Points, float64(size), rot),
				}
				lookup[strands[i].Ports[0]] = portRef{Strand: i, End: 0}
				lookup[strands[i].Ports[1]] = portRef{Strand: i, End: 1}
			}
			rotations[rot] = shape{
				Strands:    strands,
				PortLookup: lookup,
			}
		}
		res[kind] = rotations
	}
	return res
}

func baseStrands(kind TileKind, lanes int, size int, arcSegments int) []strand {
	res := make([]strand, 0, lanes)
	sz := float64(size)
	for i := 0; i < lanes; i++ {
		slot := slotPosition(i, lanes, sz)
		switch kind {
		case TileLine:
			res = append(res, strand{
				Ports: [2]port{
					{Edge: edgeWest, Slot: i},
					{Edge: edgeEast, Slot: i},
				},
				Points: []point{
					{X: 0, Y: slot},
					{X: sz, Y: slot},
				},
			})
		case TileArc:
			res = append(res, strand{
				Ports: [2]port{
					{Edge: edgeSouth, Slot: i},
					{Edge: edgeWest, Slot: i},
				},
				Points: quarterArc(point{X: 0, Y: 0}, slot, 0, math.Pi/2, arcSegments),
			})
		}
	}
	return res
}

func slotPosition(idx int, lanes int, size float64) float64 {
	return (float64(idx) + 1) * size / float64(lanes+1)
}

func quarterArc(center point, radius float64, startAngle float64, endAngle float64, segments int) []point {
	res := make([]point, 0, segments+1)
	for i := 0; i <= segments; i++ {
		t := float64(i) / float64(segments)
		angle := startAngle + (endAngle-startAngle)*t
		res = append(res, point{
			X: center.X + radius*math.Cos(angle),
			Y: center.Y + radius*math.Sin(angle),
		})
	}
	return res
}

func rotatePoints(in []point, size float64, rot int) []point {
	if rot%4 == 0 {
		res := make([]point, len(in))
		copy(res, in)
		return res
	}

	res := make([]point, len(in))
	for i, pt := range in {
		res[i] = rotatePoint(pt, size, rot)
	}
	return res
}

func rotatePoint(p point, size float64, rot int) point {
	res := p
	for i := 0; i < rot%4; i++ {
		res = point{X: res.Y, Y: size - res.X}
	}
	return res
}

func rotatePort(p port, lanes int, rot int) port {
	res := p
	for i := 0; i < rot%4; i++ {
		switch res.Edge {
		case edgeNorth:
			res = port{Edge: edgeEast, Slot: lanes - 1 - res.Slot}
		case edgeEast:
			res = port{Edge: edgeSouth, Slot: res.Slot}
		case edgeSouth:
			res = port{Edge: edgeWest, Slot: lanes - 1 - res.Slot}
		case edgeWest:
			res = port{Edge: edgeNorth, Slot: res.Slot}
		}
	}
	return res
}

func solve(g Grid, opts Opts, shapes map[TileKind][4]shape) ([]int, int, []int) {
	rng := rand.New(rand.NewSource(opts.Seed))
	cellCount := g.Rows * g.Cols
	bestRotations := make([]int, cellCount)
	bestScore := -1
	bestSizes := []int(nil)
	bestSeen := 0

	for restart := 0; restart < opts.SearchRestarts; restart++ {
		rotations := randomRotations(g, rng, opts.LockedRotations)

		score, _ := scoreLayout(g, opts.Lanes, rotations, shapes)
		order := make([]int, cellCount)
		for i := range order {
			order[i] = i
		}

		for pass := 0; pass < opts.SearchPasses; pass++ {
			changed := false
			improved := false
			rng.Shuffle(len(order), func(i int, j int) {
				order[i], order[j] = order[j], order[i]
			})

			for _, idx := range order {
				if _, locked := opts.LockedRotations[idx]; locked {
					continue
				}
				current := rotations[idx]
				bestLocalScore := score
				bestLocalRots := []int{current}
				allowed := append([]int(nil), allowedRotations(g.Tiles[idx])...)
				rng.Shuffle(len(allowed), func(i int, j int) {
					allowed[i], allowed[j] = allowed[j], allowed[i]
				})

				for _, cand := range allowed {
					if cand == current {
						continue
					}
					rotations[idx] = cand
					candScore, _ := scoreLayout(g, opts.Lanes, rotations, shapes)
					switch {
					case candScore > bestLocalScore:
						bestLocalScore = candScore
						bestLocalRots = []int{cand}
					case candScore == bestLocalScore:
						bestLocalRots = append(bestLocalRots, cand)
					}
				}

				choice := bestLocalRots[rng.Intn(len(bestLocalRots))]
				rotations[idx] = choice
				if choice != current {
					changed = true
				}
				if bestLocalScore > score {
					score = bestLocalScore
					improved = true
				}
			}

			if !improved && !changed {
				break
			}
		}

		score, sizes := scoreLayout(g, opts.Lanes, rotations, shapes)
		switch {
		case score > bestScore:
			bestScore = score
			bestSizes = sizes
			copy(bestRotations, rotations)
			bestSeen = 1
		case score == bestScore:
			bestSeen++
			if rng.Intn(bestSeen) == 0 {
				bestSizes = sizes
				copy(bestRotations, rotations)
			}
		}
	}

	return bestRotations, bestScore, bestSizes
}

func randomRotations(g Grid, rng *rand.Rand, locked map[int]int) []int {
	rotations := make([]int, len(g.Tiles))
	for i, kind := range g.Tiles {
		if locked != nil {
			if rot, ok := locked[i]; ok {
				rotations[i] = rot
				continue
			}
		}
		allowed := allowedRotations(kind)
		rotations[i] = allowed[rng.Intn(len(allowed))]
	}
	return rotations
}

func allowedRotations(kind TileKind) []int {
	switch kind {
	case TileLine:
		return []int{0, 1}
	default:
		return []int{0, 1, 2, 3}
	}
}

func scoreLayout(g Grid, lanes int, rotations []int, shapes map[TileKind][4]shape) (int, []int) {
	totalStrands := g.Rows * g.Cols * lanes
	uf := newUnionFind(totalStrands)

	for row := 0; row < g.Rows; row++ {
		for col := 0; col < g.Cols; col++ {
			idx := g.index(row, col)
			cur := shapes[g.At(row, col)][rotations[idx]]
			base := idx * lanes

			if col+1 < g.Cols {
				rightIdx := g.index(row, col+1)
				right := shapes[g.At(row, col+1)][rotations[rightIdx]]
				rightBase := rightIdx * lanes
				for slot := 0; slot < lanes; slot++ {
					a, okA := cur.PortLookup[port{Edge: edgeEast, Slot: slot}]
					b, okB := right.PortLookup[port{Edge: edgeWest, Slot: slot}]
					if okA && okB {
						uf.union(base+a.Strand, rightBase+b.Strand)
					}
				}
			}

			if row+1 < g.Rows {
				downIdx := g.index(row+1, col)
				down := shapes[g.At(row+1, col)][rotations[downIdx]]
				downBase := downIdx * lanes
				for slot := 0; slot < lanes; slot++ {
					a, okA := cur.PortLookup[port{Edge: edgeSouth, Slot: slot}]
					b, okB := down.PortLookup[port{Edge: edgeNorth, Slot: slot}]
					if okA && okB {
						uf.union(base+a.Strand, downBase+b.Strand)
					}
				}
			}
		}
	}

	counts := make(map[int]int, totalStrands)
	for i := 0; i < totalStrands; i++ {
		counts[uf.find(i)]++
	}

	sizes := make([]int, 0, len(counts))
	score := 0
	for _, size := range counts {
		sizes = append(sizes, size)
		score += size * size
	}
	sort.Ints(sizes)
	for i, j := 0, len(sizes)-1; i < j; i, j = i+1, j-1 {
		sizes[i], sizes[j] = sizes[j], sizes[i]
	}
	return score, sizes
}

func buildInstances(g Grid, opts Opts, size int, origin plot.XY, rotations []int, shapes map[TileKind][4]shape) []strandInstance {
	total := g.Rows * g.Cols * opts.Lanes
	instances := make([]strandInstance, total)
	portMaps := make([]map[port]portRef, g.Rows*g.Cols)

	for row := 0; row < g.Rows; row++ {
		for col := 0; col < g.Cols; col++ {
			cellIdx := g.index(row, col)
			tile := shapes[g.At(row, col)][rotations[cellIdx]]
			portMaps[cellIdx] = tile.PortLookup

			base := cellIdx * opts.Lanes
			offset := cellOrigin(origin, g.Rows, size, row, col)
			for strandIdx, s := range tile.Strands {
				instances[base+strandIdx] = strandInstance{
					Points: translatePoints(s.Points, offset),
				}
			}
		}
	}

	for row := 0; row < g.Rows; row++ {
		for col := 0; col < g.Cols; col++ {
			cellIdx := g.index(row, col)
			base := cellIdx * opts.Lanes

			if col+1 < g.Cols {
				rightIdx := g.index(row, col+1)
				rightBase := rightIdx * opts.Lanes
				for slot := 0; slot < opts.Lanes; slot++ {
					a, okA := portMaps[cellIdx][port{Edge: edgeEast, Slot: slot}]
					b, okB := portMaps[rightIdx][port{Edge: edgeWest, Slot: slot}]
					if okA && okB {
						connect(instances, base+a.Strand, a.End, rightBase+b.Strand, b.End)
					}
				}
			}

			if row+1 < g.Rows {
				downIdx := g.index(row+1, col)
				downBase := downIdx * opts.Lanes
				for slot := 0; slot < opts.Lanes; slot++ {
					a, okA := portMaps[cellIdx][port{Edge: edgeSouth, Slot: slot}]
					b, okB := portMaps[downIdx][port{Edge: edgeNorth, Slot: slot}]
					if okA && okB {
						connect(instances, base+a.Strand, a.End, downBase+b.Strand, b.End)
					}
				}
			}
		}
	}

	return instances
}

func cellOrigin(origin plot.XY, rows int, size int, row int, col int) plot.XY {
	return plot.XY{
		X: origin.X + col*size,
		Y: origin.Y + (rows-1-row)*size,
	}
}

func debugGridLines(g Grid, size int, origin plot.XY) []plot.Line {
	lines := make([]plot.Line, 0, g.Rows+g.Cols+2)
	x0 := origin.X
	y0 := origin.Y
	x1 := origin.X + g.Cols*size
	y1 := origin.Y + g.Rows*size

	for col := 0; col <= g.Cols; col++ {
		x := origin.X + col*size
		lines = append(lines, plot.Line{
			Start: plot.XY{X: x, Y: y0},
			End:   plot.XY{X: x, Y: y1},
		})
	}
	for row := 0; row <= g.Rows; row++ {
		y := origin.Y + row*size
		lines = append(lines, plot.Line{
			Start: plot.XY{X: x0, Y: y},
			End:   plot.XY{X: x1, Y: y},
		})
	}

	return lines
}

func debugTileGlyph(kind TileKind, rotation int, size int, offset plot.XY) []plot.Line {
	switch kind {
	case TileLine:
		margin := max(8, size/4)
		local := rotatePoints([]point{
			{X: float64(margin), Y: float64(size) / 2},
			{X: float64(size - margin), Y: float64(size) / 2},
		}, float64(size), rotation)
		return quantizePath(translatePoints(local, offset))
	case TileArc:
		radius := math.Min(float64(size)*0.28, float64(size-max(8, size/5)))
		local := rotatePoints(quarterArc(point{X: 0, Y: 0}, radius, 0, math.Pi/2, 8), float64(size), rotation)
		return quantizePath(translatePoints(local, offset))
	default:
		return nil
	}
}

func connect(instances []strandInstance, a int, aPort int, b int, bPort int) {
	instances[a].Neighbors[aPort] = neighborRef{Strand: b, Port: bPort, Valid: true}
	instances[b].Neighbors[bPort] = neighborRef{Strand: a, Port: aPort, Valid: true}
}

func translatePoints(in []point, offset plot.XY) []point {
	res := make([]point, len(in))
	for i, pt := range in {
		res[i] = point{
			X: pt.X + float64(offset.X),
			Y: pt.Y + float64(offset.Y),
		}
	}
	return res
}

func tracePaths(instances []strandInstance) [][]point {
	visited := make([]bool, len(instances))
	res := make([][]point, 0, len(instances))

	for idx := range instances {
		if visited[idx] {
			continue
		}
		if !instances[idx].Neighbors[0].Valid || !instances[idx].Neighbors[1].Valid {
			startPort := 0
			if instances[idx].Neighbors[0].Valid && !instances[idx].Neighbors[1].Valid {
				startPort = 1
			}
			res = append(res, traceFrom(idx, startPort, instances, visited))
		}
	}

	for idx := range instances {
		if visited[idx] {
			continue
		}
		res = append(res, traceFrom(idx, 0, instances, visited))
	}

	sort.Slice(res, func(i int, j int) bool {
		return len(res[i]) > len(res[j])
	})
	return res
}

func traceFrom(startStrand int, startPort int, instances []strandInstance, visited []bool) []point {
	var path []point
	curStrand := startStrand
	entryPort := startPort

	for {
		if visited[curStrand] {
			break
		}

		visited[curStrand] = true
		inst := instances[curStrand]
		pts := inst.Points
		exitPort := 1
		if entryPort == 1 {
			pts = reversePoints(pts)
			exitPort = 0
		}
		path = appendPath(path, pts)

		next := inst.Neighbors[exitPort]
		if !next.Valid || visited[next.Strand] {
			break
		}

		curStrand = next.Strand
		entryPort = next.Port
	}

	return path
}

func reversePoints(in []point) []point {
	res := make([]point, len(in))
	for i := range in {
		res[len(in)-1-i] = in[i]
	}
	return res
}

func appendPath(dst []point, src []point) []point {
	if len(src) == 0 {
		return dst
	}
	if len(dst) == 0 {
		return append(dst, src...)
	}
	if almostEqualPoint(dst[len(dst)-1], src[0]) {
		return append(dst, src[1:]...)
	}
	return append(dst, src...)
}

func quantizePaths(paths [][]point) [][]plot.Line {
	res := make([][]plot.Line, 0, len(paths))
	for _, path := range paths {
		lines := quantizePath(path)
		if len(lines) == 0 {
			continue
		}
		res = append(res, lines)
	}
	return res
}

func quantizePath(path []point) []plot.Line {
	if len(path) < 2 {
		return nil
	}

	res := make([]plot.Line, 0, len(path)-1)
	prev := roundPoint(path[0])
	for _, pt := range path[1:] {
		next := roundPoint(pt)
		if prev == next {
			continue
		}
		res = append(res, plot.Line{Start: prev, End: next})
		prev = next
	}
	return res
}

func roundPoint(p point) plot.XY {
	return plot.XY{
		X: int(math.Round(p.X)),
		Y: int(math.Round(p.Y)),
	}
}

func almostEqualPoint(a point, b point) bool {
	return math.Abs(a.X-b.X) < 1e-6 && math.Abs(a.Y-b.Y) < 1e-6
}

func (g Grid) index(row int, col int) int {
	return row*g.Cols + col
}

type unionFind struct {
	parent []int
	size   []int
}

func newUnionFind(n int) *unionFind {
	parent := make([]int, n)
	size := make([]int, n)
	for i := range parent {
		parent[i] = i
		size[i] = 1
	}
	return &unionFind{parent: parent, size: size}
}

func (u *unionFind) find(x int) int {
	if u.parent[x] != x {
		u.parent[x] = u.find(u.parent[x])
	}
	return u.parent[x]
}

func (u *unionFind) union(a int, b int) {
	ra := u.find(a)
	rb := u.find(b)
	if ra == rb {
		return
	}
	if u.size[ra] < u.size[rb] {
		ra, rb = rb, ra
	}
	u.parent[rb] = ra
	u.size[ra] += u.size[rb]
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
