package live

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
)

//go:embed index.html
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
		tmpdir:     tmpdir,
		customArgs: customArgs,
		clients:    make(map[chan event]struct{}),
	}

	if stat.IsDir() {
		if _, err := os.Stat(filepath.Join(fn, "main.go")); err == nil {
			s.singleFile = fn
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
	Type string `json:"type"` // "ready", "building", "error"
	File string `json:"file,omitempty"`
	Msg  string `json:"msg,omitempty"`
}

type server struct {
	tmpdir     string
	customArgs []string

	singleFile string
	galleryDir string

	mu          sync.Mutex
	outFile     string             // last rendered output path
	current     string             // current sketch name (gallery mode)
	stopWatch   chan struct{}       // stops the current file watcher
	cancelBuild context.CancelFunc // cancels in-flight build
	lastEvent   event              // last event for new SSE clients
	clients     map[chan event]struct{}
}

// broadcast sends an event to all connected SSE clients and caches it
func (s *server) broadcast(e event) {
	s.mu.Lock()
	s.lastEvent = e
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
	s.mu.Unlock()

	s.broadcast(event{Type: "building"})

	out, err := execute(ctx, s.tmpdir, fn, s.customArgs)
	cancel()
	if err != nil {
		msg := err.Error()
		if ctx.Err() != nil {
			msg = "build timed out"
		}
		log.WithError(err).WithField("fn", fn).Error("build failed")
		s.broadcast(event{Type: "error", Msg: msg})
		return
	}

	s.mu.Lock()
	s.outFile = out
	s.mu.Unlock()

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
		state := struct {
			Sketch string `json:"sketch"`
			Event  event  `json:"event"`
		}{
			Sketch: s.current,
			Event:  s.lastEvent,
		}
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(state)
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

func execute(ctx context.Context, tmpdir, fn string, customArgs []string) (outFN string, err error) {
	outFN = filepath.Join(tmpdir, fmt.Sprintf("%d.png", time.Now().UnixMilli()))

	var dir, base string
	if stat, err := os.Stat(fn); err != nil {
		return "", err
	} else if stat.IsDir() {
		dir = fn
		base = "main.go"
	} else {
		dir = filepath.Dir(fn)
		base = filepath.Base(fn)
	}

	var args []string
	args = append(args, "run", base, "--output", outFN)
	args = append(args, customArgs...)

	log.WithField("outFN", outFN).WithField("args", args).Info("executing go-pen program")

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return outFN, cmd.Run()
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
