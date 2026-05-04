package contour

import (
	"fmt"
	"math"

	"github.com/csweichel/go-pen/pkg/plot"
)

type Opts struct {
	// Lines is the number of contour lanes to generate around the centerline.
	Lines int
	// Spacing is the distance between neighboring lanes in plot units.
	Spacing float64
	// SamplesPerSpan controls how finely each control-point span is sampled.
	SamplesPerSpan int
	// CapSegments controls how many segments are used for the rounded end turns.
	CapSegments int
	// Tension controls spline tightness. 0 is classic Catmull-Rom, 1 tightens
	// into piecewise easing with minimal overshoot.
	Tension float64
}

type point struct {
	X float64
	Y float64
}

func Continuous(control []plot.XY, opts Opts) ([]plot.Line, error) {
	if len(control) < 2 {
		return nil, fmt.Errorf("need at least two control points")
	}

	opts, err := normalizeOpts(opts)
	if err != nil {
		return nil, err
	}

	center, err := sampleControlPath(control, opts)
	if err != nil {
		return nil, err
	}
	if len(center) < 2 {
		return nil, fmt.Errorf("sampled path is too short")
	}

	tangents, normals, err := frames(center)
	if err != nil {
		return nil, err
	}

	offsets := laneOffsets(opts.Lines, opts.Spacing)
	path := make([]point, 0, len(center)*len(offsets)+opts.CapSegments*len(offsets))

	for laneIdx, off := range offsets {
		lane := offsetLane(center, normals, off)
		if laneIdx%2 == 0 {
			path = appendPath(path, lane)
		} else {
			path = appendPath(path, reversePoints(lane))
		}

		if laneIdx == len(offsets)-1 {
			continue
		}

		if laneIdx%2 == 0 {
			cap := roundCap(
				center[len(center)-1],
				tangents[len(tangents)-1],
				normals[len(normals)-1],
				off,
				offsets[laneIdx+1],
				1,
				opts.CapSegments,
			)
			path = appendPath(path, cap)
		} else {
			cap := roundCap(
				center[0],
				tangents[0],
				normals[0],
				off,
				offsets[laneIdx+1],
				-1,
				opts.CapSegments,
			)
			path = appendPath(path, cap)
		}
	}

	return quantize(path), nil
}

func normalizeOpts(opts Opts) (Opts, error) {
	if opts.Lines < 0 {
		return Opts{}, fmt.Errorf("lines must be positive")
	}
	if opts.Spacing < 0 {
		return Opts{}, fmt.Errorf("spacing must be positive")
	}
	if opts.SamplesPerSpan < 0 {
		return Opts{}, fmt.Errorf("samples per span must be positive")
	}
	if opts.CapSegments < 0 {
		return Opts{}, fmt.Errorf("cap segments must be positive")
	}

	if opts.Lines == 0 {
		opts.Lines = 12
	}
	if opts.Spacing == 0 {
		opts.Spacing = 8
	}
	if opts.SamplesPerSpan == 0 {
		opts.SamplesPerSpan = 32
	}
	if opts.CapSegments == 0 {
		opts.CapSegments = 12
	}

	opts.Tension = clamp01(opts.Tension)
	return opts, nil
}

func sampleControlPath(control []plot.XY, opts Opts) ([]point, error) {
	points := dedupeControlPoints(control)
	if len(points) < 2 {
		return nil, fmt.Errorf("need at least two distinct control points")
	}
	if len(points) == 2 {
		return sampleLinear(points[0], points[1], opts.SamplesPerSpan), nil
	}

	res := make([]point, 0, len(points)*opts.SamplesPerSpan)
	res = append(res, points[0])
	for i := 0; i < len(points)-1; i++ {
		p0 := points[max(i-1, 0)]
		p1 := points[i]
		p2 := points[i+1]
		p3 := points[min(i+2, len(points)-1)]
		for step := 1; step <= opts.SamplesPerSpan; step++ {
			t := float64(step) / float64(opts.SamplesPerSpan)
			res = append(res, catmullRom(p0, p1, p2, p3, t, opts.Tension))
		}
	}
	return res, nil
}

func dedupeControlPoints(control []plot.XY) []point {
	res := make([]point, 0, len(control))
	for _, p := range control {
		fp := point{X: float64(p.X), Y: float64(p.Y)}
		if len(res) > 0 && distance(res[len(res)-1], fp) < 1e-6 {
			continue
		}
		res = append(res, fp)
	}
	return res
}

func sampleLinear(a, b point, samples int) []point {
	res := make([]point, 0, samples+1)
	res = append(res, a)
	for i := 1; i <= samples; i++ {
		t := float64(i) / float64(samples)
		res = append(res, lerpPoint(a, b, t))
	}
	return res
}

func catmullRom(p0, p1, p2, p3 point, t float64, tension float64) point {
	t2 := t * t
	t3 := t2 * t
	scale := 0.5 * (1 - tension)

	m1 := point{
		X: (p2.X - p0.X) * scale,
		Y: (p2.Y - p0.Y) * scale,
	}
	m2 := point{
		X: (p3.X - p1.X) * scale,
		Y: (p3.Y - p1.Y) * scale,
	}

	h00 := 2*t3 - 3*t2 + 1
	h10 := t3 - 2*t2 + t
	h01 := -2*t3 + 3*t2
	h11 := t3 - t2

	return point{
		X: h00*p1.X + h10*m1.X + h01*p2.X + h11*m2.X,
		Y: h00*p1.Y + h10*m1.Y + h01*p2.Y + h11*m2.Y,
	}
}

func frames(path []point) ([]point, []point, error) {
	tangents := make([]point, len(path))
	normals := make([]point, len(path))

	for i := range path {
		prev := path[max(i-1, 0)]
		next := path[min(i+1, len(path)-1)]
		tangent := normalize(point{
			X: next.X - prev.X,
			Y: next.Y - prev.Y,
		})
		if tangent == (point{}) {
			return nil, nil, fmt.Errorf("path contains a zero-length tangent at sample %d", i)
		}
		tangents[i] = tangent
		normals[i] = point{X: -tangent.Y, Y: tangent.X}
	}

	return tangents, normals, nil
}

func laneOffsets(lines int, spacing float64) []float64 {
	res := make([]float64, lines)
	center := float64(lines-1) * 0.5
	for i := 0; i < lines; i++ {
		res[i] = (float64(i) - center) * spacing
	}
	return res
}

func offsetLane(center []point, normals []point, off float64) []point {
	res := make([]point, len(center))
	for i := range center {
		res[i] = center[i].add(normals[i].mul(off))
	}
	return res
}

func reversePoints(in []point) []point {
	res := make([]point, len(in))
	for i := range in {
		res[len(in)-1-i] = in[i]
	}
	return res
}

func roundCap(endpoint point, tangent point, normal point, fromOff float64, toOff float64, direction float64, segments int) []point {
	if almostEqual(fromOff, toOff) {
		return []point{endpoint.add(normal.mul(toOff))}
	}

	mid := 0.5 * (fromOff + toOff)
	radius := 0.5 * math.Abs(toOff-fromOff)

	startAngle := math.Pi
	endAngle := 0.0
	if toOff < fromOff {
		startAngle = 0
		endAngle = math.Pi
	}

	res := make([]point, 0, segments+1)
	for i := 0; i <= segments; i++ {
		t := float64(i) / float64(segments)
		angle := startAngle + (endAngle-startAngle)*t
		off := mid + radius*math.Cos(angle)
		axial := direction * radius * math.Sin(angle)
		res = append(res, endpoint.add(normal.mul(off)).add(tangent.mul(axial)))
	}
	return res
}

func appendPath(dst []point, src []point) []point {
	for _, p := range src {
		if len(dst) > 0 && distance(dst[len(dst)-1], p) < 1e-6 {
			continue
		}
		dst = append(dst, p)
	}
	return dst
}

func quantize(path []point) []plot.Line {
	ints := make([]plot.XY, 0, len(path))
	for _, p := range path {
		q := plot.XY{
			X: int(math.Round(p.X)),
			Y: int(math.Round(p.Y)),
		}
		if len(ints) > 0 && ints[len(ints)-1] == q {
			continue
		}
		ints = append(ints, q)
	}

	lines := make([]plot.Line, 0, max(0, len(ints)-1))
	for i := 1; i < len(ints); i++ {
		if ints[i-1] == ints[i] {
			continue
		}
		lines = append(lines, plot.Line{
			Start: ints[i-1],
			End:   ints[i],
		})
	}
	return lines
}

func lerpPoint(a, b point, t float64) point {
	return point{
		X: a.X + (b.X-a.X)*t,
		Y: a.Y + (b.Y-a.Y)*t,
	}
}

func normalize(p point) point {
	length := math.Hypot(p.X, p.Y)
	if length == 0 {
		return point{}
	}
	return point{X: p.X / length, Y: p.Y / length}
}

func distance(a, b point) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func (p point) add(other point) point {
	return point{X: p.X + other.X, Y: p.Y + other.Y}
}

func (p point) mul(scale float64) point {
	return point{X: p.X * scale, Y: p.Y * scale}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
