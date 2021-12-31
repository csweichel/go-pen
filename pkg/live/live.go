package live

import (
	"embed"
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

// Serve starts to serve a live-preview of the file from fn at the given addr.
// fn is expected to be a Go file which uses plotr.Run
func Serve(fn string, addr string) error {
	reload := make(chan string, 1)
	stop := make(chan struct{})

	tmpdir, err := os.MkdirTemp("", "plotr-*")
	if err != nil {
		return err
	}

	out, err := execute(tmpdir, fn)
	if err != nil {
		return err
	}
	reload <- out

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go serve(l, reload)

	changes, err := watchFile(stop, fn)
	if err != nil {
		return err
	}

	go func() {
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
				res, err := execute(tmpdir, fn)
				if err != nil {
					log.WithError(err).Error("cannot execute plotr program")
				}
				reload <- res
			case <-stop:
				return
			}
		}
	}()

	log.WithField("addr", addr).Info("serving live preview")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	close(stop)

	return nil
}

func serve(l net.Listener, reload <-chan string) {
	server := socketio.NewServer(nil)
	server.OnConnect("/", func(c socketio.Conn) error {
		log.WithField("client", c.ID()).Info("client connected")
		return nil
	})
	server.OnDisconnect("/", func(c socketio.Conn, reason string) {
		log.WithField("client", c.ID()).WithField("reason", reason).Info("client disconnected")
	})
	go server.Serve()
	defer server.Close()

	var (
		mu sync.Mutex
		fn string
	)
	go func() {
		for f := range reload {
			mu.Lock()
			fn = f
			mu.Unlock()

			server.BroadcastToNamespace("/", "reload", filepath.Base(f))
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/out/", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		http.ServeFile(rw, r, fn)
	}))
	mux.Handle("/socket.io/", server)
	mux.Handle("/", http.FileServer(http.FS(index)))
	http.Serve(l, mux)
}

func execute(tmpdir, fn string) (outFN string, err error) {
	outFN = filepath.Join(tmpdir, fmt.Sprintf("%d.png", time.Now().UnixMilli()))
	log.WithField("outFN", outFN).Info("executing plotr program")

	cmd := exec.Command("go", "run", fn, "--output", outFN)
	cmd.Dir = filepath.Dir(fn)
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
