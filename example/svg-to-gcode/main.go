package main

import (
	"io"
	"os"

	"github.com/csweichel/go-pen/pkg/plot"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

func main() {
	var (
		input       = pflag.StringP("input", "i", "", "Path to the input SVG file")
		output      = pflag.StringP("output", "o", "", "Path to the output G-code file, or - for stdout")
		deviceOpts  = pflag.String("device-opts", "", "Path to the G-code device option file")
		gcodeFlavor = pflag.String("gcode-flavor", "", "G-code flavor override. Available: vanilla, mk4s")
	)
	pflag.Parse()

	if *input == "" || *output == "" {
		pflag.Usage()
		os.Exit(2)
	}

	var out io.Writer
	if *output == "-" {
		out = os.Stdout
	} else {
		f, err := os.OpenFile(*output, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			log.WithError(err).Fatal("cannot open output file")
		}
		defer f.Close()
		out = f
	}

	if err := plot.ConvertSVGToGCodeWithVpype(out, *input, *deviceOpts, *gcodeFlavor); err != nil {
		log.WithError(err).Fatal("cannot convert svg to gcode")
	}
}
