package plot

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	log "github.com/sirupsen/logrus"
)

func hasVpype() bool {
	_, err := exec.LookPath("vpype")
	return err == nil
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
	cmd := exec.Command("vpype", args...)
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
