package main

import (
	"math"

	"github.com/csweichel/go-pen/pkg/field"
	"github.com/csweichel/go-pen/pkg/plot"
)

func main() {
	plot.Run(plot.Canvas{
		Size:  plot.A4.Mult(4),
		Bleed: plot.XY{X: 15, Y: 40},
	}, func(p plot.Canvas, args map[string]string) (d plot.Drawing, err error) {
		const (
			imagePath    = "../assets/sample.png"
			lineCount    = 80
			stepsPerLine = 600
			maxAmplitude = 5.5  // max vertical oscillation in canvas units
			maxFreq      = 0.12 // max oscillation frequency (cycles per canvas unit)
		)

		counts := plot.XY{X: stepsPerLine, Y: lineCount}
		imgField, err := field.NewImageField(imagePath, p, counts)
		if err != nil {
			return nil, err
		}

		debug := args["debug"] == "true"

		if debug {
			d = append(d, plot.AsDebug(imgField.Draw(p.Bleed)...)...)
		}

		inner := p.Inner()
		dy := float64(inner.Y) / float64(lineCount)
		dx := float64(inner.X) / float64(stepsPerLine)

		for line := 0; line < lineCount; line++ {
			baseY := float64(line) * dy
			var prev plot.XY

			// Accumulate phase so the sine wave is continuous along the line,
			// with frequency varying per-sample.
			phase := 0.0

			for step := 0; step <= stepsPerLine; step++ {
				x := float64(step) * dx

				sample := imgField.Nearest(plot.XY{X: int(x), Y: int(baseY)})
				darkness := 0.0
				if sample != nil {
					darkness = 1.0 - sample.Luminosity
				}

				// Frequency and amplitude both scale with darkness.
				freq := darkness * maxFreq
				amp := darkness * maxAmplitude

				// Advance phase by frequency × step width.
				phase += freq * dx * 2 * math.Pi

				offset := amp * math.Sin(phase)

				cur := plot.XY{
					X: int(x) + p.Bleed.X,
					Y: int(baseY+offset) + p.Bleed.Y,
				}

				if step > 0 {
					d = append(d, plot.Line{Start: prev, End: cur})
				}
				prev = cur
			}
		}

		if debug {
			d = append(d, plot.AsDebug(p.Frame()...)...)
			d = append(d, plot.AsDebug(p.FrameBleed()...)...)
		}

		return d, nil
	})
}
