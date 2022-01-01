package main

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/csweichel/go-pen/pkg/live"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name: "gopen",
		Commands: []*cli.Command{
			{
				Name:      "preview",
				Usage:     "starts a live preview of a go-pen program",
				ArgsUsage: "<filename>",
				Action: func(c *cli.Context) error {
					log.SetLevel(log.DebugLevel)

					fn := c.Args().First()
					if fn == "" {
						cli.ShowAppHelpAndExit(c, 128)
					}

					const addr = "0.0.0.0:9999"
					return live.Serve(fn, addr)
				},
			},
			{
				Name:      "init",
				Usage:     "creates a new sketch",
				ArgsUsage: "<path>",
				Action:    initSketch,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "pkg-name",
						Value: "sketch",
						Usage: "Go package name of the sketch",
					},
				},
			},
		},
		Usage: "CLI for working with go-pen",
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func initSketch(c *cli.Context) error {
	fn := c.Args().First()
	if fn == "" {
		cli.ShowAppHelpAndExit(c, 128)
	}
	if _, err := os.Stat(fn); err == nil {
		log.Fatalf("%s exists already", fn)
	}

	err := os.MkdirAll(fn, 0755)
	if err != nil {
		return err
	}

	dl, err := http.Get("https://raw.githubusercontent.com/csweichel/go-pen/main/example/hello-world/main.go")
	if err != nil {
		return err
	}
	defer dl.Body.Close()

	f, err := os.OpenFile(filepath.Join(fn, "main.go"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, dl.Body)
	if err != nil {
		return err
	}

	cmds := [][]string{
		{"go", "mod", "init", c.String("pkg-name")},
		{"go", "mod", "tidy"},
	}
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = fn
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	return nil
}
