![logo](logo.png)

go-plot is a simple generative art framework for pen plotter. It supports
- [X] live-reload/preview of plotter programs
- [X] basic geometries: lines, arcs and bezier curves
- [X] vector fields, including perlin noise generated ones
- [X] PNG output
- [ ] SVG output
- [ ] Gcode output

## Try it out

## Getting started
```bash
# install goplot CLI
go install github.com/csweichel/go-plot/cmd/goplot@latest

# create a new sketch
mkdir my-sketches
goplot init my-sketches/hello-world

# start live-preview
goplot preview my-sketches/hello-world/main.go
```
