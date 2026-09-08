package host_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/alternayte/avero/config"
	"github.com/alternayte/avero/host"
)

// recorder collects the lifecycle events of a test in order.
type recorder struct {
	mu     sync.Mutex
	events []string
}

func (r *recorder) add(event string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recorder) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

func (r *recorder) equal(t *testing.T, want ...string) {
	t.Helper()
	got := r.all()
	if len(got) != len(want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("events = %v, want %v", got, want)
		}
	}
}

// fake is a component that records its calls. It does not implement Ready.
type fake struct {
	name         string
	rec          *recorder
	startErr     error
	stopErr      error
	startBlocks  chan struct{}
	panicOnStart bool

	mu           sync.Mutex
	stopDeadline time.Time
	stopHasLimit bool
}

func newFake(rec *recorder, name string) *fake { return &fake{name: name, rec: rec} }

func (f *fake) Name() string { return f.name }

func (f *fake) Start(ctx context.Context) error {
	f.rec.add("start " + f.name)
	if f.panicOnStart {
		panic("start " + f.name + " panicked")
	}
	if f.startBlocks != nil {
		select {
		case <-f.startBlocks:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return f.startErr
}

func (f *fake) Stop(ctx context.Context) error {
	f.rec.add("stop " + f.name)
	f.mu.Lock()
	f.stopDeadline, f.stopHasLimit = ctx.Deadline()
	f.mu.Unlock()
	return f.stopErr
}

func (f *fake) deadline() (time.Time, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopDeadline, f.stopHasLimit
}

// readyFake is a component that also reports its readiness.
type readyFake struct {
	*fake
	mu  sync.Mutex
	err error
}

func newReadyFake(rec *recorder, name string) *readyFake {
	return &readyFake{fake: newFake(rec, name)}
}

func (r *readyFake) Ready(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

func (r *readyFake) setReady(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}

// migrator records that it ran.
type migrator struct {
	rec *recorder
	err error
}

func (m *migrator) Migrate(context.Context) error {
	m.rec.add("migrate")
	return m.err
}

// baseConfig returns a configuration with a short grace, so that a shutdown
// test finishes quickly.
func baseConfig(grace time.Duration) config.BaseConfig {
	return config.BaseConfig{
		Port:          0,
		LogLevel:      "error",
		LogFormat:     "text",
		ShutdownGrace: grace,
		Env:           "test",
	}
}

// quietLogger returns a logger that writes nothing. A test asserts behaviour,
// not log output.
func quietLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

// listener binds a port that the operating system chooses.
func listener(t *testing.T) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned an error: %v", err)
	}
	return l
}

// newApp builds an application with the test defaults: no signal handler, a
// quiet logger and a listener on a port that the operating system chooses.
func newApp(t *testing.T, cfg config.BaseConfig, opts ...host.Option) *host.App {
	t.Helper()
	base := []host.Option{
		host.WithoutSignals(),
		host.WithLogger(quietLogger()),
		host.WithListener(listener(t)),
	}
	return host.New(cfg, append(base, opts...)...)
}

// run starts the application and returns a function that closes it and
// returns the result of Run. The test cleanup calls it, so a test that fails
// early never leaves the application running.
func run(t *testing.T, app *host.App) func() error {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	errs := make(chan error, 1)
	go func() { errs <- app.Run(ctx) }()

	var (
		once     sync.Once
		result   error
		finished = make(chan struct{})
	)
	stop := func() error {
		once.Do(func() {
			cancel()
			select {
			case result = <-errs:
			case <-time.After(20 * time.Second):
				result = errors.New("Run did not return within 20 seconds")
			}
			close(finished)
		})
		<-finished
		return result
	}
	t.Cleanup(func() { _ = stop() })
	return stop
}

// runOnce runs the application with a context that expires. A test that
// expects a fault before the wait must not hang when the fault disappears.
func runOnce(t *testing.T, app *host.App) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	err := app.Run(ctx)
	if err == nil && ctx.Err() != nil {
		t.Fatal("Run reached the wait, but the test expected a fault before the start")
	}
	return err
}

// get performs one GET against the running application.
func get(t *testing.T, app *host.App, path string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "http://"+app.Addr()+path, nil)
	if err != nil {
		t.Fatalf("NewRequest returned an error: %v", err)
	}
	res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("GET %s returned an error: %v", path, err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("ReadAll returned an error: %v", err)
	}
	return res.StatusCode, string(body)
}

// waitFor polls until cond holds. It fails the test at the deadline.
func waitFor(t *testing.T, why string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out while waiting for %s", why)
}

// waitReady polls /readyz until it answers 200.
func waitReady(t *testing.T, app *host.App) {
	t.Helper()
	waitFor(t, "the readiness gate to open", func() bool {
		res, err := (&http.Client{Timeout: time.Second}).Get("http://" + app.Addr() + "/readyz")
		if err != nil {
			return false
		}
		defer func() { _ = res.Body.Close() }()
		_, _ = io.Copy(io.Discard, res.Body)
		return res.StatusCode == http.StatusOK
	})
}

// waitServing polls /readyz until the server answers at all, whatever the code.
func waitServing(t *testing.T, app *host.App) {
	t.Helper()
	waitFor(t, "the HTTP server to listen", func() bool {
		res, err := (&http.Client{Timeout: time.Second}).Get("http://" + app.Addr() + "/readyz")
		if err != nil {
			return false
		}
		defer func() { _ = res.Body.Close() }()
		_, _ = io.Copy(io.Discard, res.Body)
		return true
	})
}

var errFake = errors.New("the fake component failed")
