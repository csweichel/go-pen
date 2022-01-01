![logo](logo.png)

go-pen is a simple generative art framework for pen plotter. It supports
- [X] live-reload/preview of plotter programs
- [X] basic geometries: lines, arcs and bezier curves
- [X] vector fields, including perlin noise generated ones
- [X] PNG output
- [ ] SVG output
- [ ] Gcode output

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
