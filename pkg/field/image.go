package field

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"

	"github.com/csweichel/go-pen/pkg/plot"
)

// Sample holds the HSL color data for a single grid point sampled from an image.
type Sample struct {
	Hue        float64 // 0–360 degrees
	Saturation float64 // 0–1
	Luminosity float64 // 0–1
}

// ImageField is a grid of HSL samples taken from a raster image, mapped onto a canvas.
type ImageField struct {
	Samples   [][]Sample
	Spacing   plot.XY // integer spacing, used by Draw for cell positioning
	SpacingF  [2]float64 // precise float spacing for Nearest lookups
	Counts    plot.XY
}

// NewImageField loads a PNG or JPEG image, resamples it to the given grid resolution,
// and converts each sampled pixel to HSL. The grid maps to the canvas inner area
// (respecting bleed), stretching the image to fill.
func NewImageField(imagePath string, canvas plot.Canvas, counts plot.XY) (*ImageField, error) {
	f, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("open image: %w", err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	bounds := img.Bounds()
	imgW := bounds.Max.X - bounds.Min.X
	imgH := bounds.Max.Y - bounds.Min.Y

	inner := canvas.Inner()
	dx := float64(inner.X) / float64(counts.X)
	dy := float64(inner.Y) / float64(counts.Y)
	spacing := plot.XY{X: int(dx), Y: int(dy)}

	samples := make([][]Sample, counts.X)
	for x := 0; x < counts.X; x++ {
		samples[x] = make([]Sample, counts.Y)
		for y := 0; y < counts.Y; y++ {
			// Map grid position to image pixel (nearest-neighbor).
			// The canvas coordinate system has Y=0 at the bottom (Y increases upward),
			// but image pixels have Y=0 at the top, so we flip the Y mapping.
			imgX := bounds.Min.X + int(float64(x)*float64(imgW)/float64(counts.X))
			imgY := bounds.Min.Y + int(float64(counts.Y-1-y)*float64(imgH)/float64(counts.Y))
			if imgX >= bounds.Max.X {
				imgX = bounds.Max.X - 1
			}
			if imgY >= bounds.Max.Y {
				imgY = bounds.Max.Y - 1
			}

			r, g, b, _ := img.At(imgX, imgY).RGBA()
			// RGBA returns 16-bit values, normalize to 0–1
			rf := float64(r) / 65535.0
			gf := float64(g) / 65535.0
			bf := float64(b) / 65535.0

			h, s, l := rgbToHSL(rf, gf, bf)
			samples[x][y] = Sample{Hue: h, Saturation: s, Luminosity: l}
		}
	}

	return &ImageField{
		Samples:  samples,
		Spacing:  spacing,
		SpacingF: [2]float64{dx, dy},
		Counts:   counts,
	}, nil
}

// Nearest returns the nearest sample for a canvas coordinate (relative to the
// inner area, i.e. after subtracting bleed). Returns nil if out of bounds.
func (f *ImageField) Nearest(p plot.XY) *Sample {
	x := int(float64(p.X) / f.SpacingF[0])
	y := int(float64(p.Y) / f.SpacingF[1])

	if x < 0 || x >= len(f.Samples) {
		return nil
	}
	col := f.Samples[x]
	if y < 0 || y >= len(col) {
		return nil
	}
	return &col[y]
}

// Draw produces a debug overlay that visually approximates the source image.
// Each grid cell is filled with horizontal lines whose density is proportional
// to the darkness of the sample (low luminosity = more lines).
func (f *ImageField) Draw(offset plot.XY) []plot.Drawable {
	const maxLinesPerCell = 8

	var res []plot.Drawable
	for x := 0; x < len(f.Samples); x++ {
		for y := 0; y < len(f.Samples[x]); y++ {
			s := f.Samples[x][y]
			// Darkness: 0 = white (no lines), 1 = black (max lines)
			darkness := 1.0 - s.Luminosity
			lineCount := int(math.Round(darkness * maxLinesPerCell))
			if lineCount <= 0 {
				continue
			}

			cellX := x*f.Spacing.X + offset.X
			cellY := y*f.Spacing.Y + offset.Y

			for i := 0; i < lineCount; i++ {
				ly := cellY + int(float64(i+1)*float64(f.Spacing.Y)/float64(lineCount+1))
				res = append(res, plot.Line{
					Start: plot.XY{X: cellX, Y: ly},
					End:   plot.XY{X: cellX + f.Spacing.X, Y: ly},
				})
			}
		}
	}
	return res
}

// rgbToHSL converts RGB values (each 0–1) to HSL.
// Returns hue in 0–360, saturation and luminosity in 0–1.
func rgbToHSL(r, g, b float64) (h, s, l float64) {
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	l = (max + min) / 2.0

	if max == min {
		// Achromatic
		return 0, 0, l
	}

	d := max - min
	if l > 0.5 {
		s = d / (2.0 - max - min)
	} else {
		s = d / (max + min)
	}

	switch max {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	case b:
		h = (r-g)/d + 4
	}
	h *= 60

	return h, s, l
}
