package dev_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alternayte/avero/cmd/avero/dev"
)

// child is an application that the loop starts and stops.
type child struct {
	server *httptest.Server
	body   *atomic.Pointer[string]
	stops  *atomic.Int32
}

// Stop ends the application.
func (c child) Stop() error {
	c.stops.Add(1)
	c.server.Close()
	return nil
}

// loop returns a server with a fake application, a fake CSS build and a fake
// Go build. Every seam is a field, so a test needs no compiler and no
// Tailwind binary. See design rule 3.
func loop(t *testing.T, dir string) (*dev.Server, *atomic.Pointer[string], *atomic.Int32, *atomic.Int32) {
	t.Helper()
	body := &atomic.Pointer[string]{}
	page := "<html><body><h1>one</h1></body></html>"
	body.Store(&page)
	builds := &atomic.Int32{}
	stops := &atomic.Int32{}

	var current *httptest.Server
	t.Cleanup(func() {
		if current != nil {
			current.Close()
		}
	})
	port := freePort(t)

	server, err := dev.New(dev.Config{
		Dir:       dir,
		Env:       dev.EnvDevelopment,
		ChildPort: port,
		AssetDir:  filepath.Join(dir, "assets", "dist"),
		BuildCSS: func(context.Context) (string, error) {
			builds.Add(1)
			return "/assets/css/app.beef.css", nil
		},
		BuildGo: func(context.Context) error {
			builds.Add(1)
			return nil
		},
		Start: func(_ context.Context, port int) (dev.Process, error) {
			// The fake application listens on the port of the loop, as the
			// real one does.
			l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
			if err != nil {
				return nil, err
			}
			srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write([]byte(*body.Load()))
			}))
			srv.Listener = l
			srv.Start()
			current = srv
			return child{server: srv, body: body, stops: stops}, nil
		},
	})
	if err != nil {
		t.Fatalf("New returned %v, want nil", err)
	}
	return server, body, builds, stops
}

// listen opens the reload channel and returns the events that arrive.
func listen(t *testing.T, h http.Handler) (<-chan map[string]string, func()) {
	t.Helper()
	events := make(chan map[string]string, 8)
	req := httptest.NewRequest(http.MethodGet, dev.ReloadPath, nil)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	rec := newStreamRecorder()

	go h.ServeHTTP(rec, req)
	go func() {
		reader := bufio.NewReader(rec)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				close(events)
				return
			}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var e map[string]string
			if err := json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(line), "data: ")), &e); err == nil {
				events <- e
			}
		}
	}()
	return events, cancel
}

func TestACSSChangeReachesTheBrowserInsideTheBudget(t *testing.T) {
	// DX-2. A change to a CSS file must appear in the browser in 200
	// milliseconds, with no restart of the process and no reload of the page.
	dir := t.TempDir()
	server, _, _, stops := loop(t, dir)
	handler := server.Handler()
	if err := server.Changed(context.Background(), nil); err != nil {
		t.Fatalf("Changed returned %v, want nil", err)
	}
	events, cancel := listen(t, handler)
	defer cancel()
	waitForClient(t, server)

	start := time.Now()
	if err := server.Changed(context.Background(), []string{filepath.Join(dir, "assets", "css", "app.css")}); err != nil {
		t.Fatalf("Changed returned %v, want nil", err)
	}
	select {
	case e := <-events:
		elapsed := time.Since(start)
		if e["type"] != "css" {
			t.Fatalf("the event is %v, want a css event", e)
		}
		if e["href"] != "/assets/css/app.beef.css" {
			t.Fatalf("the event holds %q", e["href"])
		}
		if elapsed > 200*time.Millisecond {
			t.Fatalf("DX-2 took %s, and the budget is 200ms", elapsed)
		}
		t.Logf("DX-2: %s", elapsed.Round(time.Microsecond))
	case <-time.After(2 * time.Second):
		t.Fatal("no css event arrived")
	}
	if stops.Load() != 0 {
		t.Fatalf("the loop stopped the application %d times, want 0", stops.Load())
	}
}

func TestAGoChangeRebuildsAndRestartsInsideTheBudget(t *testing.T) {
	// DX-3. A change to a Go file or to a templ file must appear in the
	// browser in two seconds.
	dir := t.TempDir()
	server, body, _, stops := loop(t, dir)
	handler := server.Handler()
	if err := server.Changed(context.Background(), nil); err != nil {
		t.Fatalf("Changed returned %v, want nil", err)
	}
	if err := startChild(server); err != nil {
		t.Fatalf("the application does not start: %v", err)
	}
	events, cancel := listen(t, handler)
	defer cancel()
	waitForClient(t, server)

	next := "<html><body><h1>two</h1></body></html>"
	body.Store(&next)

	start := time.Now()
	if err := server.Changed(context.Background(), []string{filepath.Join(dir, "main.go")}); err != nil {
		t.Fatalf("Changed returned %v, want nil", err)
	}
	select {
	case e := <-events:
		if e["type"] != "reload" {
			t.Fatalf("the event is %v, want a reload event", e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no reload event arrived")
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	elapsed := time.Since(start)
	if !strings.Contains(rec.Body.String(), "two") {
		t.Fatalf("the page holds the old answer:\n%s", rec.Body.String())
	}
	if elapsed > 2*time.Second {
		t.Fatalf("DX-3 took %s, and the budget is 2s", elapsed)
	}
	if stops.Load() != 1 {
		t.Fatalf("the loop stopped the application %d times, want 1", stops.Load())
	}
	t.Logf("DX-3: %s", elapsed.Round(time.Millisecond))
}

func TestTheProxyAddsTheReloadClientToAnHTMLAnswer(t *testing.T) {
	dir := t.TempDir()
	server, _, _, _ := loop(t, dir)
	if err := startChild(server); err != nil {
		t.Fatalf("the application does not start: %v", err)
	}
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec.Body.String()
	if !strings.Contains(body, dev.ReloadPath) {
		t.Fatalf("the answer carries no reload client:\n%s", body)
	}
	if strings.Index(body, "<script>") > strings.Index(body, "</body>") {
		t.Fatalf("the client stands after the body:\n%s", body)
	}
}

func TestTheReloadClientFetchesAndMorphs(t *testing.T) {
	// The client fetches the page and morphs the document. It never navigates,
	// so the page keeps its scroll position, its focus and the state of its
	// elements across a restart. See S15.
	script := string(dev.Client())
	for _, want := range []string{"fetch(window.location.href", "morph(", "DOMParser"} {
		if !strings.Contains(script, want) {
			t.Fatalf("the client holds no %q", want)
		}
	}
	for _, absent := range []string{"location.reload", "window.location =", "location.href ="} {
		if strings.Contains(script, absent) {
			t.Fatalf("the client navigates with %q, which loses the scroll position", absent)
		}
	}
}

func TestTheProxyHoldsARequestWhileTheApplicationRestarts(t *testing.T) {
	// The browser keeps its page across a restart, because the request waits
	// instead of failing.
	dir := t.TempDir()
	server, body, _, _ := loop(t, dir)
	handler := server.Handler()
	if err := startChild(server); err != nil {
		t.Fatalf("the application does not start: %v", err)
	}
	next := "<html><body><h1>three</h1></body></html>"

	done := make(chan string, 1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		done <- rec.Body.String()
	}()

	body.Store(&next)
	if err := server.Changed(context.Background(), []string{filepath.Join(dir, "main.go")}); err != nil {
		t.Fatalf("Changed returned %v, want nil", err)
	}
	select {
	case page := <-done:
		if !strings.Contains(page, "three") {
			t.Fatalf("the request did not wait for the new process:\n%s", page)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the request did not answer")
	}
}

func TestTheProxyAddsNothingOutsideDevelopment(t *testing.T) {
	server, err := dev.New(dev.Config{Dir: t.TempDir(), Env: "production"})
	if err != nil {
		t.Fatalf("New returned %v, want nil", err)
	}
	if err := server.Run(context.Background()); err == nil {
		t.Fatal("Run accepted an environment that is not development")
	} else if !strings.Contains(err.Error(), "AVERO_ENV") {
		t.Fatalf("the fault is %v", err)
	}

	child := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body>page</body></html>"))
	}))
	defer child.Close()
	port := portOf(t, child.URL)
	production, err := dev.New(dev.Config{Dir: t.TempDir(), Env: "production", ChildPort: port})
	if err != nil {
		t.Fatalf("New returned %v, want nil", err)
	}
	rec := httptest.NewRecorder()
	production.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if strings.Contains(rec.Body.String(), dev.ReloadPath) {
		t.Fatalf("the production proxy added the reload client:\n%s", rec.Body.String())
	}
}

func TestTheProxyServesTheAssetsFromTheDisk(t *testing.T) {
	// The binary of the application embeds the assets of its build, so the
	// loop serves the new stylesheet from the disk. See DX-2.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets", "dist", "css"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "dist", "css", "app.beef.css"),
		[]byte("h1{color:red}"), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	server, _, _, _ := loop(t, dir)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/css/app.beef.css", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "color:red") {
		t.Fatalf("the loop answered %d:\n%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
}

// startChild starts the fake application of a test.
func startChild(server *dev.Server) error {
	return server.Changed(context.Background(), []string{"main.go"})
}

// waitForClient waits until the reload channel holds one browser.
func waitForClient(t *testing.T, server *dev.Server) {
	t.Helper()
	for range 200 {
		if server.Clients() > 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("no browser reached the reload channel")
}

// freePort returns a port that no process holds.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned %v", err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

// portOf returns the port of a test server address.
func portOf(t *testing.T, address string) int {
	t.Helper()
	var host string
	var port int
	if _, err := fmt.Sscanf(strings.TrimPrefix(address, "http://"), "%s", &host); err != nil {
		t.Fatalf("Sscanf returned %v", err)
	}
	parts := strings.Split(host, ":")
	if _, err := fmt.Sscanf(parts[len(parts)-1], "%d", &port); err != nil {
		t.Fatalf("the address %q holds no port", address)
	}
	return port
}

func TestTheSweepFindsAChangeThatArrivedDuringABuild(t *testing.T) {
	// A notification that arrives while a build runs can reach a library
	// buffer that nothing reads. The loop reads the tree after each build, so
	// no change waits for the next save.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	server, _, _, _ := loop(t, dir)

	started := time.Now()
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\n// changed\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	files := server.Sweep(started)
	if len(files) != 1 || filepath.Base(files[0]) != "main.go" {
		t.Fatalf("the sweep found %v, want main.go", files)
	}
	if later := server.Sweep(time.Now()); len(later) != 0 {
		t.Fatalf("the sweep found %v after the change, want none", later)
	}
}

func TestTheWatcherReportsAWriteThatCarriesAModeChange(t *testing.T) {
	// os.WriteFile writes the content and the mode, and macOS reports
	// WRITE|CHMOD for it. A filter that drops every event with the mode bit
	// loses the change, and the page then holds the old answer.
	dir := t.TempDir()
	name := filepath.Join(dir, "main.go")
	if err := os.WriteFile(name, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}

	server, _, _, _ := loop(t, dir)
	changed := make(chan []string, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		files, err := server.Watch(ctx)
		if err != nil {
			return
		}
		changed <- files
	}()

	// The watcher needs a moment to reach the tree.
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(name, []byte("package main\n\n// changed\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}

	select {
	case files := <-changed:
		if len(files) != 1 || filepath.Base(files[0]) != "main.go" {
			t.Fatalf("the watcher reported %v, want main.go", files)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the watcher reported no change")
	}
}
