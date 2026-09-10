// Package dev holds the development loop, S15.
//
// `avero dev` runs a proxy in front of the application. The proxy serves the
// reload channel at /_avero/reload, and it adds the reload client to an HTML
// answer. The application therefore carries no development code, and a
// production build holds none of it.
//
// A change to a CSS file rebuilds the stylesheet and sends the new address to
// the browser. The page swaps the link element, so no process restarts and no
// page reloads. See DX-2.
//
// A change to a Go file or to a templ file rebuilds the binary and starts it
// again. The proxy holds the request until the new process answers, so the
// browser keeps its page and its scroll position. See DX-3.
package dev

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// hint returns the repair line of a fault.
//
// An error carries one sentence that states the repair. That sentence ends
// with a full stop. The linter refuses an error literal that ends with
// punctuation, so the sentence arrives as a value. See DX-7.
func hint(sentence string) string { return "\n  → " + sentence }

// debugWatch prints each change that the watcher reports. A test of the loop
// sets AVERO_DEV_DEBUG to read it.
var debugWatch = os.Getenv("AVERO_DEV_DEBUG") != ""

// The defaults of the loop.
const (
	// ReloadPath is the address of the reload channel.
	ReloadPath = "/_avero/reload"
	// HealthPath is the address that the proxy polls after a restart.
	HealthPath = "/healthz"
	// StartTimeout bounds the wait for a child that starts.
	StartTimeout = 30 * time.Second
	// RequestWait bounds the wait of one request during a restart.
	RequestWait = 10 * time.Second
	// EnvDevelopment is the environment that carries the loop.
	EnvDevelopment = "development"
)

// Config states one development loop.
type Config struct {
	// Dir is the root of the application.
	Dir string
	// Port is the port that the browser reads.
	Port int
	// ChildPort is the port of the application. A zero value takes a free
	// port.
	ChildPort int
	// Env states the environment. The loop runs in development only.
	Env string
	// Out receives the log of the loop and the output of the application.
	Out io.Writer
	// BuildCSS rebuilds the stylesheet and returns its address. A nil value
	// builds with the asset pipeline of the project.
	BuildCSS func(ctx context.Context) (string, error)
	// BuildScript rebuilds the bundle of the front end. A nil value builds
	// with the asset pipeline of the project.
	BuildScript func(ctx context.Context) error
	// BuildGo rebuilds the binary of the application. A nil value runs the
	// Go tool.
	BuildGo func(ctx context.Context) error
	// Start starts the application and returns the process. A nil value runs
	// the binary that BuildGo wrote.
	Start func(ctx context.Context, port int) (Process, error)
	// FrontEnd runs the development server of the front end, such as Vite. A
	// nil value runs none, and the loop then owns the assets itself.
	FrontEnd []string
	// FrontEndDir is the directory of that command, relative to Dir. The
	// watcher skips it, because the development server of the front end
	// watches it.
	FrontEndDir string
	// AssetDir holds the built assets. The proxy serves them from the disk,
	// because the binary of the application carries the copy of the last
	// build. A CSS change therefore reaches the browser with no restart.
	// See DX-2.
	AssetDir string
	// AssetBase is the URL prefix of the assets. An empty value is
	// /assets/.
	AssetBase string
}

// Process is one running application. The loop stops it and starts it again.
type Process interface {
	// Stop ends the process and waits for it.
	Stop() error
}

// ExitAware is the optional contract of a process that reports its own end.
// The loop states the fault at once instead of waiting for the health
// endpoint. See DX-7.
type ExitAware interface {
	// Exited returns a channel that carries the end of the process.
	Exited() <-chan error
}

// Server is the development loop. It watches the tree, rebuilds and proxies.
type Server struct {
	cfg   Config
	hub   *hub
	proxy *proxy

	mu      sync.Mutex
	child   Process
	restart chan struct{}
}

// New builds the loop.
func New(cfg Config) (*Server, error) {
	if cfg.Dir == "" {
		cfg.Dir = "."
	}
	if cfg.Env == "" {
		cfg.Env = EnvDevelopment
	}
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}
	if cfg.ChildPort == 0 {
		port, err := freePort()
		if err != nil {
			return nil, err
		}
		cfg.ChildPort = port
	}
	s := &Server{cfg: cfg, hub: newHub(), restart: make(chan struct{}, 1)}
	s.proxy = newProxy(cfg.ChildPort, cfg.Env == EnvDevelopment)
	return s, nil
}

// Watch returns the source files of the first change of the tree. A test reads
// it. The loop runs the same watcher.
func (s *Server) Watch(ctx context.Context) ([]string, error) {
	w, err := newWatcher(s.cfg.Dir, s.cfg.FrontEndDir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = w.Close() }()
	return w.next(ctx)
}

// Sweep returns the source files of the tree that changed after a time. The
// loop calls it after each rebuild, and a test reads it.
func (s *Server) Sweep(since time.Time) []string {
	w, err := newWatcher(s.cfg.Dir, s.cfg.FrontEndDir)
	if err != nil {
		return nil
	}
	defer func() { _ = w.Close() }()
	return w.sweep(since)
}

// Clients returns the number of browsers that listen on the reload channel.
func (s *Server) Clients() int { return s.hub.Clients() }

// Addr returns the address that the browser reads.
func (s *Server) Addr() string { return fmt.Sprintf("127.0.0.1:%d", s.cfg.Port) }

// Handler returns the handler of the proxy. A test drives it with no listener.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle(ReloadPath, s.hub)
	if s.cfg.AssetDir != "" {
		base := s.cfg.AssetBase
		if base == "" {
			base = "/assets/"
		}
		// The proxy serves the built assets from the disk. The binary of the
		// application embeds the assets of its build, so a stylesheet that
		// the loop rebuilt would not reach the browser through it.
		mux.Handle(base, http.StripPrefix(base, noCache(http.FileServer(http.Dir(s.cfg.AssetDir)))))
	}
	mux.Handle("/", s.proxy)
	return mux
}

// noCache stops the browser from holding an asset of the loop.
func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// Run starts the application, watches the tree and serves the proxy. It
// returns when the caller cancels the context.
func (s *Server) Run(ctx context.Context) error {
	if s.cfg.Env != EnvDevelopment {
		return fmt.Errorf("avero dev: AVERO_ENV is %q\n  → Set AVERO_ENV=development, because the loop rebuilds and restarts the application",
			s.cfg.Env)
	}
	if err := s.build(ctx); err != nil {
		return err
	}
	if err := s.start(ctx); err != nil {
		return err
	}
	defer s.stop()

	// templ owns its files. The loop wraps `go tool templ generate --watch`,
	// which writes the Go file of each templ file. The watcher then sees the
	// Go file and rebuilds.
	templ, err := StartTempl(ctx, s.cfg.Dir, s.cfg.Out)
	if err != nil {
		return err
	}
	if templ != nil {
		_, _ = fmt.Fprintln(s.cfg.Out, "avero dev: templ generate --watch")
		defer func() { _ = templ.Stop() }()
	}

	// The front end owns its own loop. Vite watches its files, replaces a
	// module in the page and proxies the API to this application.
	front, err := s.startFrontEnd(ctx)
	if err != nil {
		return err
	}
	if front != nil {
		defer func() { _ = front.Stop() }()
	}

	server := &http.Server{
		Addr:              s.Addr(),
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()

	go s.watch(ctx)

	_, _ = fmt.Fprintf(s.cfg.Out, "avero dev: http://%s\n", s.Addr())
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Changed answers one change of the tree. A test calls it with no watcher.
func (s *Server) Changed(ctx context.Context, files []string) error {
	switch kind := classify(files); kind {
	case changeCSS:
		href, err := s.buildCSS(ctx)
		if err != nil {
			s.hub.send(event{Type: "fault", Message: err.Error()})
			return err
		}
		s.hub.send(event{Type: "css", Href: href})
		return nil
	case changeScript:
		start := time.Now()
		if err := s.buildScript(ctx); err != nil {
			s.hub.send(event{Type: "fault", Message: err.Error()})
			return err
		}
		// A module that already runs cannot be replaced, so the page loads
		// again. It is the one change that reloads. The build is the whole
		// cost, and esbuild takes tens of milliseconds.
		s.hub.send(event{Type: "script"})
		_, _ = fmt.Fprintf(s.cfg.Out, "avero dev: script %s\n", time.Since(start).Round(time.Millisecond))
		return nil
	case changeGo:
		start := time.Now()
		s.proxy.hold()
		if err := s.build(ctx); err != nil {
			s.proxy.release()
			s.hub.send(event{Type: "fault", Message: err.Error()})
			return err
		}
		built := time.Now()
		s.stop()
		if err := s.start(ctx); err != nil {
			s.proxy.release()
			return err
		}
		s.proxy.release()
		s.hub.send(event{Type: "reload"})
		_, _ = fmt.Fprintf(s.cfg.Out, "avero dev: reload %s, of which the restart is %s\n",
			time.Since(start).Round(time.Millisecond), time.Since(built).Round(time.Millisecond))
		return nil
	default:
		return nil
	}
}

// buildCSS rebuilds the stylesheet and returns its address.
func (s *Server) buildCSS(ctx context.Context) (string, error) {
	if s.cfg.BuildCSS == nil {
		return "", errors.New("avero dev: the loop holds no CSS build\n  → Write the entry of the stylesheet in avero.json")
	}
	return s.cfg.BuildCSS(ctx)
}

// buildScript rebuilds the bundle of the front end.
func (s *Server) buildScript(ctx context.Context) error {
	if s.cfg.BuildScript == nil {
		return errors.New("avero dev: the loop holds no script build\n  → Name the entry of the script in avero.json")
	}
	return s.cfg.BuildScript(ctx)
}

// build rebuilds the binary of the application.
func (s *Server) build(ctx context.Context) error {
	if s.cfg.BuildGo == nil {
		return nil
	}
	start := time.Now()
	if err := s.cfg.BuildGo(ctx); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(s.cfg.Out, "avero dev: build %s\n", time.Since(start).Round(time.Millisecond))
	return nil
}

// start starts the application and waits for its health endpoint.
func (s *Server) start(ctx context.Context) error {
	if s.cfg.Start == nil {
		return nil
	}
	child, err := s.cfg.Start(ctx, s.cfg.ChildPort)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.child = child
	s.mu.Unlock()
	return s.waitForChild(ctx, child)
}

// stop ends the application.
func (s *Server) stop() {
	s.mu.Lock()
	child := s.child
	s.child = nil
	s.mu.Unlock()
	if child != nil {
		_ = child.Stop()
	}
}

// waitForChild polls the health endpoint of the application. It returns at
// once when the application ends.
func (s *Server) waitForChild(ctx context.Context, child Process) error {
	var exited <-chan error
	if aware, ok := child.(ExitAware); ok {
		exited = aware.Exited()
	}
	address := fmt.Sprintf("http://127.0.0.1:%d%s", s.cfg.ChildPort, HealthPath)
	deadline := time.Now().Add(StartTimeout)
	client := &http.Client{Timeout: time.Second}
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
		if err != nil {
			return err
		}
		res, err := client.Do(req)
		if err == nil {
			_ = res.Body.Close()
			return nil
		}
		select {
		case failure := <-exited:
			return failure
		case <-time.After(5 * time.Millisecond):
		}
	}
	return fmt.Errorf("avero dev: the application did not answer on %s%s", address,
		hint("Read the output of the application. Repair the fault that it names."))
}

// startFrontEnd runs the development server of the front end.
func (s *Server) startFrontEnd(ctx context.Context) (Process, error) {
	if len(s.cfg.FrontEnd) == 0 {
		return nil, nil
	}
	dir := filepath.Join(s.cfg.Dir, filepath.FromSlash(s.cfg.FrontEndDir))
	cmd := exec.CommandContext(ctx, s.cfg.FrontEnd[0], s.cfg.FrontEnd[1:]...)
	cmd.Dir = dir
	cmd.Stdout = s.cfg.Out
	cmd.Stderr = s.cfg.Out
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("AVERO_API_URL=http://127.0.0.1:%d", s.cfg.ChildPort))
	Group(cmd)
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("avero dev: the front end does not start: %w%s", err,
			hint("Install Node.js. Run `avero dev` again."))
	}
	_, _ = fmt.Fprintf(s.cfg.Out, "avero dev: %s in %s\n",
		strings.Join(s.cfg.FrontEnd, " "), s.cfg.FrontEndDir)
	return stopper{stop: func() error {
		KillGroup(cmd)
		_ = cmd.Wait()
		return nil
	}}, nil
}

// watch answers each change of the tree.
func (s *Server) watch(ctx context.Context) {
	w, err := newWatcher(s.cfg.Dir, s.cfg.FrontEndDir)
	if err != nil {
		_, _ = fmt.Fprintf(s.cfg.Out, "%v\n", err)
		return
	}
	defer func() { _ = w.Close() }()

	for {
		files, err := w.next(ctx)
		if err != nil {
			return
		}
		for len(files) > 0 {
			if debugWatch {
				_, _ = fmt.Fprintf(s.cfg.Out, "avero dev: change %v\n", files)
			}
			start := time.Now()
			if err := s.Changed(ctx, files); err != nil {
				_, _ = fmt.Fprintf(s.cfg.Out, "avero dev: %v\n", err)
			}
			// A change that arrives during the build must not wait for the
			// next notification, so the loop reads the tree one time.
			files = w.sweep(start)
			if err := ctx.Err(); err != nil {
				return
			}
		}
	}
}

// The kinds of change that the loop answers.
type changeKind int

const (
	changeNone changeKind = iota
	changeCSS
	changeScript
	changeGo
)

// classify returns the kind of one change.
//
// A Go change wins, because it rebuilds the binary and every other output
// follows it. A script change rebuilds the bundle only, and a stylesheet
// change rebuilds the stylesheet only.
func classify(files []string) changeKind {
	kind := changeNone
	for _, name := range files {
		switch strings.ToLower(filepath.Ext(name)) {
		case ".go", ".templ", ".sql", ".json":
			return changeGo
		case ".js", ".jsx", ".ts", ".tsx":
			kind = changeScript
		case ".css":
			if kind == changeNone {
				kind = changeCSS
			}
		}
	}
	return kind
}

// freePort returns a port that no process holds.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("avero dev: no port is free: %w%s", err,
			hint("Close a process that holds every port. Run the command again."))
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}
