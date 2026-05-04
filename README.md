![logo](logo.png)
<img src="logo.jpeg" style="width: 300px" />

go-pen is a simple generative art framework for pen plotter. It supports
- [X] live-reload/preview of plotter programs
- [X] basic geometries: lines, arcs and bezier curves
- [X] vector fields, including perlin noise generated ones
- [X] PNG output
- [x] SVG output
- [x] Gcode output

## Try it out
[![Open in Gitpod](https://gitpod.io/button/open-in-gitpod.svg)](https://gitpod.io/#github.com/csweichel/go-pen)

## Getting started
```bash
# install goplot CLI
go install github.com/csweichel/go-pen/cmd/gopen@latest

# create a new sketch
mkdir my-sketches
gopen init my-sketches/hello-world

# start live-preview
gopen preview my-sketches/hello-world/main.go
```

## Generate gcode
All go-pen sketches are self-contained Go programs and can be executed as such. To generate gcode from a sketch just run that sketch:
```
# print the CLI help
go run example/field/main.go --help

# generate gcode
# Tip: inspecting the gcode is easy with https://icesl.loria.fr/webprinter/
go run example/field/main.go --output field.gcode --device gcode --device-opts example/gcode-opts.json

# generate Prusa-oriented gcode without editing the device opts JSON
go run example/field/main.go --output field-mk4s.gcode --device gcode --gcode-flavor mk4s
```

Notice the `--device-opts` flag which enables output device configuration. For gcode, the `GCodeOpts` struct in `pkg/plot/gcode.go` defines the available options, including the optional `flavor` field. You can also override the JSON setting at runtime with `--gcode-flavor vanilla` or `--gcode-flavor mk4s`.

If you also enable `--optimise vpype` with `--device gcode`, go-pen will try to route the export through vpype's `gwrite` command. This requires the `vpype-gcode` plug-in. If that plug-in is not installed, go-pen falls back to its native G-code generator.
