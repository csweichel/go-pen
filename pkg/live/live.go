package live

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/csweichel/go-pen/pkg/plot"
	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
)

//go:generate go run ../../cmd/build-live-ui
//go:embed index.html app.js
var index embed.FS

const buildTimeout = 60 * time.Second

// Serve starts a live-preview server at addr.
//
// If fn is a file, it previews that single sketch with file watching.
// If fn is a directory, it discovers sketch subdirectories (containing main.go)
// and serves a gallery UI where the user picks which sketch to run.
func Serve(fn string, addr string, customArgs []string) error {
	fn, err := filepath.Abs(fn)
	if err != nil {
		return err
	}

	stat, err := os.Stat(fn)
	if err != nil {
		return err
	}

	tmpdir, err := os.MkdirTemp("", "go-pen-*")
	if err != nil {
		return err
	}

	s := &server{
		tmpdir:            tmpdir,
		customArgs:        customArgs,
		clients:           make(map[chan event]struct{}),
		argProfiles:       make(map[string]argProfile),
		interactiveStates: make(map[string]json.RawMessage),
		profileLoaded:     make(map[string]bool),
		vpypeAvail:        hasVpype(),
	}

	if stat.IsDir() {
		if _, err := os.Stat(filepath.Join(fn, "main.go")); err == nil {
			s.singleFile = filepath.Join(fn, "main.go")
		} else {
			s.galleryDir = fn
		}
	} else {
		s.singleFile = fn
	}

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go s.serveHTTP(l)

	// In single-file mode, build the first image and start watching
	if s.singleFile != "" {
		s.build(s.singleFile)
		stop := make(chan struct{})
		s.mu.Lock()
		s.stopWatch = stop
		s.mu.Unlock()
		changes, err := watchFile(stop, s.singleFile)
		if err != nil {
			return err
		}
		go s.watchLoop(s.singleFile, changes, stop)
	}

	log.WithField("addr", addr).WithField("target", fn).Info("serving live preview")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	return nil
}

// event is pushed to SSE clients
type event struct {
	Type string `json:"type"` // "ready", "building", "error", "log"
	File string `json:"file,omitempty"`
	Msg  string `json:"msg,omitempty"`
	Log  string `json:"log,omitempty"`
}

type server struct {
	tmpdir     string
	customArgs []string

	singleFile string
	galleryDir string
	vpypeAvail bool

	mu                sync.Mutex
	outFile           string             // last rendered output path
	current           string             // current sketch name (gallery mode)
	stopWatch         chan struct{}      // stops the current file watcher
	cancelBuild       context.CancelFunc // cancels in-flight build
	lastEvent         event              // last event for new SSE clients
	buildLog          []string
	optimiseLLO       bool
	optimiseVP        bool
	showDebug         bool
	clients           map[chan event]struct{}
	argProfiles       map[string]argProfile
	interactiveStates map[string]json.RawMessage
	profileLoaded     map[string]bool
}

// broadcast sends an event to all connected SSE clients and caches it
func (s *server) broadcast(e event) {
	s.mu.Lock()
	if e.Type != "log" {
		s.lastEvent = e
	}
	for ch := range s.clients {
		select {
		case ch <- e:
		default: // drop if client is slow
		}
	}
	s.mu.Unlock()
}

func (s *server) addClient() chan event {
	ch := make(chan event, 8)
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	// Send current state immediately
	if s.lastEvent.Type != "" {
		ch <- s.lastEvent
	}
	s.mu.Unlock()
	return ch
}

func (s *server) removeClient(ch chan event) {
	s.mu.Lock()
	delete(s.clients, ch)
	s.mu.Unlock()
}

func (s *server) listSketches() []string {
	entries, err := os.ReadDir(s.galleryDir)
	if err != nil {
		log.WithError(err).Error("cannot list sketches")
		return nil
	}
	var sketches []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(s.galleryDir, e.Name(), "main.go")); err == nil {
			sketches = append(sketches, e.Name())
		}
	}
	return sketches
}

func (s *server) selectSketch(name string) {
	s.mu.Lock()
	if s.stopWatch != nil {
		close(s.stopWatch)
		s.stopWatch = nil
	}
	if s.cancelBuild != nil {
		s.cancelBuild()
		s.cancelBuild = nil
	}
	s.current = name
	s.mu.Unlock()

	fn := filepath.Join(s.galleryDir, name, "main.go")
	s.build(fn)

	stop := make(chan struct{})
	s.mu.Lock()
	s.stopWatch = stop
	s.mu.Unlock()

	changes, err := watchFile(stop, fn)
	if err != nil {
		log.WithError(err).WithField("sketch", name).Error("cannot watch sketch")
		return
	}
	go s.watchLoop(fn, changes, stop)
}

func (s *server) build(fn string) {
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	s.mu.Lock()
	s.cancelBuild = cancel
	s.buildLog = nil
	s.mu.Unlock()

	s.appendBuildLog("step: build requested")
	s.appendBuildLog("step: target sketch " + fn)
	s.appendBuildLog("step: timeout " + buildTimeout.String())
	s.broadcast(event{Type: "building"})

	s.mu.Lock()
	optimiseLLO := s.optimiseLLO
	optimiseVP := s.optimiseVP
	vpypeAvail := s.vpypeAvail
	showDebug := s.showDebug
	s.mu.Unlock()

	sketchArgs := s.currentSketchArgs(fn)
	interactiveState := s.currentInteractiveState(fn)
	out, err := execute(ctx, s.tmpdir, fn, s.customArgs, sketchArgs, interactiveState, optimiseLLO, optimiseVP, vpypeAvail, showDebug, "", nil, s.appendBuildLog)
	cancel()
	if err != nil {
		msg := err.Error()
		if ctx.Err() != nil {
			msg = "build timed out"
		}
		s.appendBuildLog("step: build failed: " + msg)
		log.WithError(err).WithField("fn", fn).Error("build failed")
		s.broadcast(event{Type: "error", Msg: msg})
		return
	}

	s.mu.Lock()
	s.outFile = out
	s.mu.Unlock()

	s.appendBuildLog("step: build succeeded")
	s.appendBuildLog("step: output file " + filepath.Base(out))
	s.broadcast(event{Type: "ready", File: filepath.Base(out)})
}

func (s *server) watchLoop(fn string, changes <-chan struct{}, stop chan struct{}) {
	t := time.NewTicker(1 * time.Second)
	defer t.Stop()

	var changed bool
	for {
		select {
		case <-changes:
			changed = true
		case <-t.C:
			if !changed {
				continue
			}
			changed = false
			s.build(fn)
		case <-stop:
			return
		}
	}
}

func (s *server) serveHTTP(l net.Listener) {
	mux := http.NewServeMux()

	// SSE endpoint — pushes build state to clients
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch := s.addClient()
		defer s.removeClient(ch)

		for {
			select {
			case ev := <-ch:
				data, _ := json.Marshal(ev)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})

	// Gallery API
	mux.HandleFunc("/api/sketches", func(w http.ResponseWriter, r *http.Request) {
		if s.galleryDir == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s.listSketches())
	})

	// Select a sketch (gallery mode)
	mux.HandleFunc("/api/select", func(w http.ResponseWriter, r *http.Request) {
		if s.galleryDir == "" {
			http.NotFound(w, r)
			return
		}
		name := r.URL.Query().Get("name")
		if name == "" {
			http.Error(w, "missing name parameter", http.StatusBadRequest)
			return
		}
		log.WithField("sketch", name).Info("sketch selected")
		go s.selectSketch(name)
		w.WriteHeader(http.StatusAccepted)
	})

	// Current state
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		curFn := s.currentMainFileLocked()
		state := struct {
			Sketch string   `json:"sketch"`
			Event  event    `json:"event"`
			Logs   []string `json:"logs,omitempty"`
			Optim  struct {
				LLO        bool `json:"llo"`
				VPype      bool `json:"vpype"`
				VPypeAvail bool `json:"vpypeAvailable"`
			} `json:"optim"`
			Debug       bool             `json:"debug"`
			Args        argState         `json:"args"`
			Interactive interactiveState `json:"interactive"`
		}{
			Sketch: s.current,
			Event:  s.lastEvent,
			Logs:   append([]string(nil), s.buildLog...),
			Debug:  s.showDebug,
		}
		state.Optim.LLO = s.optimiseLLO
		state.Optim.VPype = s.optimiseVP
		state.Optim.VPypeAvail = s.vpypeAvail
		s.mu.Unlock()
		state.Args = s.argsState(curFn)
		interactiveState, err := s.interactiveState(curFn)
		if err == nil {
			state.Interactive = interactiveState
		} else {
			log.WithError(err).WithField("sketch", sketchNameForMainFile(curFn)).Warn("cannot load interactive state")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(state)
	})

	// Update optimisation toggles from the UI.
	mux.HandleFunc("/api/optimise", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			LLO   *bool `json:"llo"`
			VPype *bool `json:"vpype"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		s.mu.Lock()
		if req.LLO != nil {
			s.optimiseLLO = *req.LLO
		}
		if req.VPype != nil {
			s.optimiseVP = *req.VPype
		}
		curFn := s.currentMainFileLocked()
		s.mu.Unlock()

		if curFn != "" {
			go s.build(curFn)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			OK bool `json:"ok"`
		}{OK: true})
	})

	// Toggle debug overlay from the UI.
	mux.HandleFunc("/api/debug", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Debug *bool `json:"debug"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		s.mu.Lock()
		if req.Debug != nil {
			s.showDebug = *req.Debug
		}
		curFn := s.currentMainFileLocked()
		s.mu.Unlock()

		if curFn != "" {
			go s.build(curFn)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			OK bool `json:"ok"`
		}{OK: true})
	})

	mux.HandleFunc("/api/args", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		fn := s.currentMainFileLocked()
		s.mu.Unlock()

		if fn == "" {
			http.Error(w, "no sketch selected", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(s.argsState(fn))
		case http.MethodPost:
			var req argUpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid json body", http.StatusBadRequest)
				return
			}

			state, err := s.updateArgs(fn, req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			go s.build(fn)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(state)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/interactive", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		fn := s.currentMainFileLocked()
		s.mu.Unlock()

		if fn == "" {
			http.Error(w, "no sketch selected", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			state, err := s.interactiveState(fn)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(state)
		case http.MethodPost:
			var req interactiveUpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid json body", http.StatusBadRequest)
				return
			}

			state, err := s.updateInteractive(fn, req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			go s.build(fn)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(state)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Export the current sketch as G-code without replacing the preview asset.
	mux.HandleFunc("/api/export/gcode", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.mu.Lock()
		fn := s.currentMainFileLocked()
		sketchName := s.currentSketchNameLocked()
		optimiseLLO := s.optimiseLLO
		optimiseVP := s.optimiseVP
		vpypeAvail := s.vpypeAvail
		showDebug := s.showDebug
		s.mu.Unlock()

		if fn == "" {
			http.Error(w, "no sketch selected", http.StatusBadRequest)
			return
		}

		sketchArgs := s.currentSketchArgs(fn)
		interactiveState := s.currentInteractiveState(fn)

		flavor, err := plot.ParseGCodeFlavor(r.URL.Query().Get("flavor"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), buildTimeout)
		defer cancel()

		out, err := execute(ctx, s.tmpdir, fn, s.customArgs, sketchArgs, interactiveState, optimiseLLO, optimiseVP, vpypeAvail, showDebug, "gcode", []string{"--gcode-flavor", string(flavor)}, func(line string) {
			log.WithField("sketch", sketchName).Debug(line)
		})
		if err != nil {
			msg := err.Error()
			if ctx.Err() != nil {
				msg = "gcode export timed out"
			}
			log.WithError(err).WithField("sketch", sketchName).Error("gcode export failed")
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}

		f, err := os.Open(out)
		if err != nil {
			http.Error(w, "cannot open generated gcode", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil {
			http.Error(w, "cannot inspect generated gcode", http.StatusInternalServerError)
			return
		}

		filename := downloadFilename(sketchName, "gcode")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		http.ServeContent(w, r, filename, info.ModTime(), f)
	})

	// Serve rendered output
	mux.HandleFunc("/out/", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		out := s.outFile
		s.mu.Unlock()
		if out == "" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, out)
	})

	// UI
	mux.Handle("/", http.FileServer(http.FS(index)))

	http.Serve(l, mux)
}

func (s *server) appendBuildLog(line string) {
	if strings.TrimSpace(line) == "" {
		return
	}

	s.mu.Lock()
	const maxLogLines = 300
	s.buildLog = append(s.buildLog, line)
	if len(s.buildLog) > maxLogLines {
		s.buildLog = append([]string(nil), s.buildLog[len(s.buildLog)-maxLogLines:]...)
	}
	s.mu.Unlock()

	s.broadcast(event{Type: "log", Log: line})
}

func execute(ctx context.Context, tmpdir, fn string, customArgs []string, sketchArgs map[string]string, interactiveState json.RawMessage, optimiseLLO bool, optimiseVP bool, vpypeAvail bool, showDebug bool, forcedDevice string, extraArgs []string, onLog func(string)) (outFN string, err error) {
	if onLog == nil {
		onLog = func(string) {}
	}

	onLog("step: resolving build settings")
	renderArgs := splitManagedRenderArgs(customArgs)
	device := renderArgs.Device
	if forcedDevice != "" {
		device = forcedDevice
		onLog(fmt.Sprintf("step: overriding output device %q", forcedDevice))
	}

	if device == "" {
		device = "svg"
	}

	deviceExplicit := renderArgs.DeviceExplicit || forcedDevice != ""
	onLog(fmt.Sprintf("step: output device %q (explicit=%t)", device, deviceExplicit))
	ext := outputExtForDevice(device)
	outFN, err = newOutputPath(tmpdir, ext)
	if err != nil {
		return "", err
	}
	onLog("step: output path " + outFN)

	dir, base, err := resolveSketchEntrypoint(fn)
	if err != nil {
		return "", err
	}
	onLog("step: working directory " + dir)
	onLog("step: entrypoint " + base)

	var args []string
	args = append(args, "run", base, "--output", outFN, "--device", device)
	statePath, cleanupState, err := writeInteractiveStateFile(tmpdir, interactiveState)
	if err != nil {
		return "", err
	}
	defer cleanupState()
	args = append(args, "--interactive-state", statePath)
	if renderArgs.DeviceOptsExplicit && renderArgs.DeviceOpts != "" {
		args = append(args, "--device-opts", renderArgs.DeviceOpts)
		onLog("step: using device opts " + renderArgs.DeviceOpts)
	}
	remainingArgs, programArgs := stripSketchArgsFlags(renderArgs.RemainingArgs)
	remainingArgs, passthroughOpts := stripManagedOptimiseFlags(remainingArgs, map[string]struct{}{
		"llo":   {},
		"vpype": {},
	})

	finalOpts := uniqueStrings(passthroughOpts)
	if optimiseLLO {
		finalOpts = append(finalOpts, "llo")
		onLog("step: enabling optimisation llo (UI)")
	}
	if optimiseVP {
		if device != "svg" && device != "gcode" {
			onLog("step: vpype optimisation requested but device does not support vpype, skipping vpype")
		} else if !vpypeAvail {
			onLog("step: vpype optimisation requested but vpype is unavailable, skipping vpype")
		} else {
			finalOpts = append(finalOpts, "vpype")
			onLog("step: enabling optimisation vpype (UI)")
		}
	}
	finalOpts = uniqueStrings(finalOpts)
	if len(finalOpts) > 0 {
		args = append(args, "--optimise", strings.Join(finalOpts, ","))
	}

	if sketchArgs != nil {
		programArgs = copyStringMap(sketchArgs)
	}
	delete(programArgs, "debug")
	if showDebug {
		if programArgs == nil {
			programArgs = make(map[string]string)
		}
		programArgs["debug"] = "true"
		onLog("step: enabling debug output (UI)")
	}
	if encoded := encodeSketchArgsValue(programArgs); encoded != "" {
		args = append(args, "--args", encoded)
		onLog("step: using sketch args " + encoded)
	}
	args = append(args, remainingArgs...)
	if len(extraArgs) > 0 {
		args = append(args, extraArgs...)
	}

	log.WithField("outFN", outFN).WithField("args", args).Info("executing go-pen program")
	onLog("step: executing command")
	onLog("command: go " + strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}
	onLog(fmt.Sprintf("step: started go process (pid=%d)", cmd.Process.Pid))

	var wg sync.WaitGroup
	wg.Add(2)
	go streamBuildOutput(&wg, stdout, "[go stdout] ", onLog)
	go streamBuildOutput(&wg, stderr, "[go stderr] ", onLog)

	err = cmd.Wait()
	wg.Wait()
	if err != nil {
		onLog("step: go process exited with error: " + err.Error())
	} else {
		onLog("step: go process completed successfully")
	}
	return outFN, err
}

func streamBuildOutput(wg *sync.WaitGroup, r io.Reader, prefix string, onLog func(string)) {
	defer wg.Done()

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := prefix + sc.Text()
		fmt.Fprintln(os.Stdout, line)
		onLog(line)
	}
	if err := sc.Err(); err != nil {
		if strings.Contains(err.Error(), "file already closed") {
			return
		}
		onLog(prefix + "log stream error: " + err.Error())
	}
}

func (s *server) currentMainFileLocked() string {
	if s.singleFile != "" {
		if st, err := os.Stat(s.singleFile); err == nil && st.IsDir() {
			return filepath.Join(s.singleFile, "main.go")
		}
		return s.singleFile
	}
	if s.galleryDir != "" && s.current != "" {
		return filepath.Join(s.galleryDir, s.current, "main.go")
	}
	return ""
}

func (s *server) currentSketchNameLocked() string {
	if s.galleryDir != "" && s.current != "" {
		return s.current
	}
	return sketchNameForMainFile(s.currentMainFileLocked())
}

type managedRenderArgs struct {
	RemainingArgs      []string
	Device             string
	DeviceExplicit     bool
	DeviceOpts         string
	DeviceOptsExplicit bool
}

func splitManagedRenderArgs(args []string) (res managedRenderArgs) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--device":
			res.DeviceExplicit = true
			if i+1 < len(args) {
				res.Device = args[i+1]
				i++
			}
		case strings.HasPrefix(a, "--device="):
			res.DeviceExplicit = true
			res.Device = strings.TrimPrefix(a, "--device=")
		case a == "--device-opts":
			res.DeviceOptsExplicit = true
			if i+1 < len(args) {
				res.DeviceOpts = args[i+1]
				i++
			}
		case strings.HasPrefix(a, "--device-opts="):
			res.DeviceOptsExplicit = true
			res.DeviceOpts = strings.TrimPrefix(a, "--device-opts=")
		case a == "--output" || a == "-o":
			if i+1 < len(args) {
				i++
			}
		case strings.HasPrefix(a, "--output="), strings.HasPrefix(a, "-o="):
			// managed output flag, ignore
		default:
			res.RemainingArgs = append(res.RemainingArgs, a)
		}
	}
	return res
}

func stripManagedOptimiseFlags(args []string, managed map[string]struct{}) (remainingArgs []string, passthroughOpts []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--optimise" || a == "-L":
			if i+1 >= len(args) {
				remainingArgs = append(remainingArgs, a)
				continue
			}
			opts := splitOptList(args[i+1])
			for _, opt := range opts {
				if _, isManaged := managed[opt]; isManaged {
					continue
				}
				passthroughOpts = append(passthroughOpts, opt)
			}
			i++
		case strings.HasPrefix(a, "--optimise="):
			opts := splitOptList(strings.TrimPrefix(a, "--optimise="))
			for _, opt := range opts {
				if _, isManaged := managed[opt]; isManaged {
					continue
				}
				passthroughOpts = append(passthroughOpts, opt)
			}
		case strings.HasPrefix(a, "-L="):
			opts := splitOptList(strings.TrimPrefix(a, "-L="))
			for _, opt := range opts {
				if _, isManaged := managed[opt]; isManaged {
					continue
				}
				passthroughOpts = append(passthroughOpts, opt)
			}
		default:
			remainingArgs = append(remainingArgs, a)
		}
	}
	return remainingArgs, passthroughOpts
}

func splitOptList(v string) []string {
	var res []string
	for _, s := range strings.Split(v, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		res = append(res, s)
	}
	return res
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	res := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		res = append(res, s)
	}
	return res
}

func outputExtForDevice(device string) string {
	switch device {
	case "svg":
		return "svg"
	case "gcode":
		return "gcode"
	case "json":
		return "json"
	case "png":
		return "png"
	default:
		return "out"
	}
}

func newOutputPath(tmpdir, ext string) (string, error) {
	pattern := "go-pen-*"
	if ext != "" {
		pattern += "." + ext
	}

	f, err := os.CreateTemp(tmpdir, pattern)
	if err != nil {
		return "", err
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		return "", err
	}
	return name, nil
}

func sketchNameForMainFile(fn string) string {
	if fn == "" {
		return "sketch"
	}

	base := filepath.Base(filepath.Clean(fn))
	if base == "." || base == string(filepath.Separator) {
		return "sketch"
	}
	if strings.EqualFold(base, "main.go") {
		parent := filepath.Base(filepath.Dir(filepath.Clean(fn)))
		if parent != "" && parent != "." && parent != string(filepath.Separator) {
			return parent
		}
	}

	name := strings.TrimSuffix(base, filepath.Ext(base))
	if name == "" {
		return "sketch"
	}
	return name
}

func downloadFilename(name string, ext string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "sketch"
	}

	var b strings.Builder
	b.Grow(len(name))

	lastDash := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
			lastDash = false
		case r == '-':
			if b.Len() > 0 && !lastDash {
				b.WriteRune(r)
				lastDash = true
			}
		default:
			if b.Len() > 0 && !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}

	base := strings.Trim(b.String(), "-_.")
	if base == "" {
		base = "sketch"
	}
	if ext == "" {
		return base
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return base + ext
}

func hasVpype() bool {
	_, err := exec.LookPath("vpype")
	return err == nil
}

func watchFile(stop chan struct{}, fn string) (changed <-chan struct{}, err error) {
	c := make(chan notify.EventInfo, 1)
	if err := notify.Watch(fn, c, notify.Write); err != nil {
		return nil, err
	}

	res := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case <-c:
				select {
				case res <- struct{}{}:
				default:
				}
			case <-stop:
				notify.Stop(c)
				return
			}
		}
	}()
	log.WithField("fn", fn).Info("watching file")
	return res, nil
}
