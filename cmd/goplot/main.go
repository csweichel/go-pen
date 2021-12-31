package main

import (
	"log"
	"os"

	"github.com/csweichel/go-plot/pkg/live"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:      "go-lot-live",
		Usage:     "Offer a live-preview of a go-lot program",
		ArgsUsage: "<filename>",
		Action: func(c *cli.Context) error {
			logrus.SetLevel(logrus.DebugLevel)

			fn := c.Args().First()
			if fn == "" {
				cli.ShowAppHelpAndExit(c, 128)
			}

			const addr = "0.0.0.0:9999"
			return live.Serve(fn, addr)
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
