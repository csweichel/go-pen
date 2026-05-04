package plot

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

var (
	execLookPath = exec.LookPath
	execCommand  = exec.Command
)

func hasVpype() bool {
	_, err := execLookPath("vpype")
	return err == nil
}

func hasVpypeGWrite() bool {
	if !hasVpype() {
		return false
	}

	cmd := execCommand("vpype", "gwrite", "--help")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

// PlotSVGWithVpype renders SVG and applies vpype post-processing if available.
// If vpype is missing or fails, it falls back to plain SVG output.
func PlotSVGWithVpype(out io.Writer, p Canvas, d Drawing) error {
	if !hasVpype() {
		fmt.Fprintln(os.Stdout, "[vpype] step: vpype binary not found, falling back to plain SVG")
		log.Warn("vpype optimisation requested but vpype is not available in PATH, using plain svg output")
		return NewSVGPlotter()(out, p, d)
	}

	fmt.Fprintln(os.Stdout, "[vpype] step: vpype available, preparing temporary files")
	tmpdir, err := os.MkdirTemp("", "go-pen-vpype-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	inFn := filepath.Join(tmpdir, "input.svg")
	optimizedFn := filepath.Join(tmpdir, "optimized.svg")

	in, err := os.Create(inFn)
	if err != nil {
		return err
	}
	if err := NewSVGPlotter()(in, p, d); err != nil {
		in.Close()
		return err
	}
	if err := in.Close(); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "[vpype] step: base SVG generated")

	args := []string{
		"read", inFn,
		"linemerge",
		"linesimplify",
		"reloop",
		"linesort",
		"write", optimizedFn,
	}
	fmt.Fprintln(os.Stdout, "[vpype] command: vpype "+joinCommand(args))
	cmd := execCommand("vpype", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "[vpype] step: started vpype process (pid=%d)\n", cmd.Process.Pid)

	var wg sync.WaitGroup
	wg.Add(2)
	go streamVpypeOutput(&wg, stdout, "[vpype stdout] ")
	go streamVpypeOutput(&wg, stderr, "[vpype stderr] ")
	err = cmd.Wait()
	wg.Wait()
	if err != nil {
		fmt.Fprintf(os.Stdout, "[vpype] step: vpype process failed: %v\n", err)
		log.WithError(err).Warn("vpype optimisation failed, using plain svg output")
		return writeFileToWriter(out, inFn)
	}
	fmt.Fprintln(os.Stdout, "[vpype] step: vpype process completed successfully")

	if _, err := os.Stat(optimizedFn); err != nil {
		fmt.Fprintln(os.Stdout, "[vpype] step: missing optimised output, falling back to plain SVG")
		log.WithError(err).Warn("vpype optimisation did not produce output, using plain svg output")
		return writeFileToWriter(out, inFn)
	}

	fmt.Fprintln(os.Stdout, "[vpype] step: writing optimised SVG")
	return writeFileToWriter(out, optimizedFn)
}

// PlotGCodeWithVpype renders SVG, applies vpype post-processing, and emits
// G-code using the vpype-gcode plug-in. If vpype or the plug-in is missing,
// or if vpype fails, it falls back to the native G-code generator.
func PlotGCodeWithVpype(out io.Writer, p Canvas, d Drawing, optFN string, flavorOverride string) error {
	opts, err := LoadGCodeOpts(optFN, flavorOverride)
	if err != nil {
		return err
	}

	fallback := newNativeGCodePlotter(opts)

	if !hasVpype() {
		fmt.Fprintln(os.Stdout, "[vpype] step: vpype binary not found, falling back to native G-code")
		log.Warn("vpype optimisation requested for gcode but vpype is not available in PATH, using native gcode output")
		return fallback(out, p, d)
	}
	if !hasVpypeGWrite() {
		fmt.Fprintln(os.Stdout, "[vpype] step: vpype-gcode plug-in not found, falling back to native G-code")
		log.Warn("vpype optimisation requested for gcode but vpype-gcode is not installed, using native gcode output")
		return fallback(out, p, d)
	}
	if opts.Scale <= 0 {
		return fmt.Errorf("gcode scale must be > 0, got %g", opts.Scale)
	}

	fmt.Fprintln(os.Stdout, "[vpype] step: vpype gwrite available, preparing temporary files")
	tmpdir, err := os.MkdirTemp("", "go-pen-vpype-gcode-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	inFn := filepath.Join(tmpdir, "input.svg")
	configFn := filepath.Join(tmpdir, "vpype-gcode.toml")
	outFn := filepath.Join(tmpdir, "output.gcode")

	in, err := os.Create(inFn)
	if err != nil {
		return err
	}
	if err := NewSVGPlotter()(in, p, d); err != nil {
		in.Close()
		return err
	}
	if err := in.Close(); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "[vpype] step: base SVG generated for gcode export")

	cfg, err := vpypeGCodeProfileConfig(opts)
	if err != nil {
		return err
	}
	if err := os.WriteFile(configFn, []byte(cfg), 0o644); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "[vpype] step: vpype gwrite profile generated")

	args := vpypeGCodeArgs(configFn, inFn, outFn, p, opts)
	fmt.Fprintln(os.Stdout, "[vpype] command: vpype "+joinCommand(args))

	cmd := execCommand("vpype", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "[vpype] step: started vpype process (pid=%d)\n", cmd.Process.Pid)

	var wg sync.WaitGroup
	wg.Add(2)
	go streamVpypeOutput(&wg, stdout, "[vpype stdout] ")
	go streamVpypeOutput(&wg, stderr, "[vpype stderr] ")
	err = cmd.Wait()
	wg.Wait()
	if err != nil {
		fmt.Fprintf(os.Stdout, "[vpype] step: vpype process failed: %v\n", err)
		log.WithError(err).Warn("vpype gcode generation failed, using native gcode output")
		return fallback(out, p, d)
	}
	fmt.Fprintln(os.Stdout, "[vpype] step: vpype process completed successfully")

	if _, err := os.Stat(outFn); err != nil {
		fmt.Fprintln(os.Stdout, "[vpype] step: missing vpype gcode output, falling back to native G-code")
		log.WithError(err).Warn("vpype gcode generation did not produce output, using native gcode output")
		return fallback(out, p, d)
	}

	fmt.Fprintln(os.Stdout, "[vpype] step: writing vpype-generated G-code")
	return writeFileToWriter(out, outFn)
}

func vpypeGCodeArgs(configFn string, inFn string, outFn string, p Canvas, opts *GCodeOpts) []string {
	scale := formatFloat(opts.Scale)
	return []string{
		"-c", configFn,
		"read", inFn,
		"translate", "--", formatFloat(float64(opts.Offset.X)), formatFloat(float64(-opts.Offset.Y)),
		"scale", "-o", "0", "0", scale, scale,
		"pagesize", vpypePageSize(p, opts.Scale),
		"linemerge",
		"linesimplify",
		"reloop",
		"linesort",
		"gwrite", "--profile", "go_pen", outFn,
	}
}

func vpypePageSize(p Canvas, scale float64) string {
	return formatFloat(scale*float64(p.Size.X)) + "x" + formatFloat(scale*float64(p.Size.Y))
}

func vpypeGCodeProfileConfig(opts *GCodeOpts) (string, error) {
	flavor, err := ParseGCodeFlavor(opts.Flavor)
	if err != nil {
		return "", err
	}

	prologue, err := gcodePrologueLines(flavor)
	if err != nil {
		return "", err
	}
	epilogue, err := gcodeEpilogueLines(flavor)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("[gwrite.go_pen]\n")
	b.WriteString(`unit = "px"` + "\n")
	b.WriteString("vertical_flip = true\n")

	if len(prologue) > 0 {
		b.WriteString("document_start = '''\n")
		b.WriteString(strings.Join(prologue, "\n"))
		b.WriteString("\n'''\n")
	}

	b.WriteString("segment_first = '''\n")
	b.WriteString(fmt.Sprintf("G0 Z%d F%d\n", opts.TravelLift, opts.TravelFeed))
	b.WriteString(fmt.Sprintf("G0 X{x:.3f} Y{y:.3f} F%d\n", opts.TravelFeed))
	b.WriteString(fmt.Sprintf("G0 Z%d F%d\n", opts.DrawLift, opts.TravelFeed))
	b.WriteString("'''\n")

	drawMove := fmt.Sprintf("G0 X{x:.3f} Y{y:.3f} F%d\\n", opts.DrawFeed)
	b.WriteString(fmt.Sprintf("segment = %q\n", drawMove))
	b.WriteString(fmt.Sprintf("segment_last = %q\n", drawMove))

	b.WriteString("line_end = '''\n")
	b.WriteString(fmt.Sprintf("G0 Z%d F%d\n", opts.TravelLift, opts.TravelFeed))
	b.WriteString("'''\n")

	if len(epilogue) > 0 {
		b.WriteString("document_end = '''\n")
		b.WriteString(strings.Join(epilogue, "\n"))
		b.WriteString("\n'''\n")
	}

	return b.String(), nil
}

func formatFloat(v float64) string {
	return fmt.Sprintf("%.12g", v)
}

func streamVpypeOutput(wg *sync.WaitGroup, r io.Reader, prefix string) {
	defer wg.Done()

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		fmt.Fprintln(os.Stdout, prefix+sc.Text())
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintln(os.Stdout, prefix+"log stream error: "+err.Error())
	}
}

func joinCommand(args []string) string {
	if len(args) == 0 {
		return ""
	}
	res := args[0]
	for i := 1; i < len(args); i++ {
		res += " " + args[i]
	}
	return res
}

func writeFileToWriter(out io.Writer, fn string) error {
	f, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(out, f); err != nil {
		return fmt.Errorf("cannot copy %s to output: %w", fn, err)
	}
	return nil
}
