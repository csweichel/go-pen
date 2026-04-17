package live

import (
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

	socketio "github.com/googollee/go-socket.io"
	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
)

//go:embed index.html
var index embed.FS

// Serve starts a live-preview server at addr.
//
// If fn is a file, it previews that single sketch with file watching.
// If fn is a directory, it discovers all sketch subdirectories (containing
// main.go) and serves a gallery UI where the user picks which sketch to run.
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
	}

	if stat.IsDir() {
		// Directory mode: gallery with sketch picker.
		// Check if this dir itself is a sketch (has main.go) or contains sketch subdirs.
		if _, err := os.Stat(filepath.Join(fn, "main.go")); err == nil {
			// Single sketch directory — treat like a file
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

	// In single-file mode, do the initial build before serving
	if s.singleFile != "" {
		out, err := execute(tmpdir, s.singleFile, customArgs)
		if err != nil {
			return err
		}
		s.outFile = out
	}

	go s.serve(l)

	// In single-file mode, start watching immediately
	if s.singleFile != "" {
		stop := make(chan struct{})
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

type server struct {
	tmpdir     string
	customArgs []string

	// Exactly one of these is set
	singleFile string // single-file mode
	galleryDir string // gallery mode

	mu        sync.Mutex
	outFile   string       // last rendered output path
	current   string       // current sketch name (gallery mode)
	stopWatch chan struct{} // stops the current file watcher
	sio       *socketio.Server
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
	s.current = name
	s.mu.Unlock()

	fn := filepath.Join(s.galleryDir, name, "main.go")

	s.buildAndBroadcast(fn)

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
			s.buildAndBroadcast(fn)
		case <-stop:
			return
		}
	}
}

func (s *server) buildAndBroadcast(fn string) {
	s.sio.BroadcastToNamespace("/", "building", "")

	out, err := execute(s.tmpdir, fn, s.customArgs)
	if err != nil {
		log.WithError(err).WithField("fn", fn).Error("build failed")
		s.sio.BroadcastToNamespace("/", "build_error", err.Error())
		return
	}

	s.mu.Lock()
	s.outFile = out
	s.mu.Unlock()

	s.sio.BroadcastToNamespace("/", "reload", filepath.Base(out))
}

func (s *server) serve(l net.Listener) {
	s.sio = socketio.NewServer(nil)
	s.sio.OnConnect("/", func(c socketio.Conn) error {
		log.WithField("client", c.ID()).Info("client connected")

		s.mu.Lock()
		out := s.outFile
		s.mu.Unlock()
		if out != "" {
			c.Emit("reload", filepath.Base(out))
		}
		return nil
	})
	s.sio.OnEvent("/", "select", func(c socketio.Conn, name string) {
		log.WithField("sketch", name).Info("sketch selected")
		go s.selectSketch(name)
	})
	s.sio.OnDisconnect("/", func(c socketio.Conn, reason string) {
		log.WithField("client", c.ID()).WithField("reason", reason).Info("client disconnected")
	})
	go s.sio.Serve()
	defer s.sio.Close()

	mux := http.NewServeMux()

	// Gallery API: returns sketch list, or 404 in single-file mode
	mux.HandleFunc("/api/sketches", func(w http.ResponseWriter, r *http.Request) {
		if s.galleryDir == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s.listSketches())
	})

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

	mux.Handle("/socket.io/", s.sio)
	mux.Handle("/", http.FileServer(http.FS(index)))

	http.Serve(l, mux)
}

func execute(tmpdir, fn string, customArgs []string) (outFN string, err error) {
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

	cmd := exec.Command("go", args...)
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
				res <- struct{}{}
			case <-stop:
				notify.Stop(c)
				return
			}
		}
	}()
	log.WithField("fn", fn).Info("watching file")
	return res, nil
}
