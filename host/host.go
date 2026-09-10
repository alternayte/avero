// Package host owns the lifecycle of an Avero application.
//
// New builds the application from a configuration and a set of options. Run
// performs the sequence that the SDD states in section 5.3: the boot checks,
// the migrations, the ordered start, the readiness gate, the wait, the drain
// and the reverse-ordered stop.
//
// Avero owns lifecycle. Avero does not own semantics. A component decides what
// Start and Stop mean.
package host

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/alternayte/avero/config"
)

// App is one Avero application. Build it with New. Run it one time.
type App struct {
	cfg        config.BaseConfig
	log        *slog.Logger
	components []Component
	checks     []Check
	handler    http.Handler
	migrator   Migrator
	signals    []os.Signal

	// open reports whether the readiness gate is open.
	open atomic.Bool
	// ran guards against a second Run.
	ran atomic.Bool
	// inFlight counts the requests inside a handler.
	inFlight atomic.Int64
	// draining reports whether the gate has closed and the drain has begun.
	draining atomic.Bool
	// idle closes when the last in-flight request ends during the drain.
	idle     chan struct{}
	idleOnce sync.Once

	mu  sync.RWMutex
	ln  net.Listener
	srv *http.Server
}

// New builds an application. Pass the BaseConfig that S1 loaded. An
// application configuration embeds it, so main.go passes cfg.BaseConfig.
func New(cfg config.BaseConfig, opts ...Option) *App {
	a := &App{
		cfg:     cfg,
		signals: []os.Signal{os.Interrupt, syscall.SIGTERM},
		idle:    make(chan struct{}),
	}
	for _, opt := range opts {
		opt(a)
	}
	if a.log == nil {
		a.log = defaultLogger(cfg)
	}
	return a
}

// Config returns the configuration that New received.
func (a *App) Config() config.BaseConfig { return a.cfg }

// Signals returns the signals that close the application.
func (a *App) Signals() []os.Signal { return a.signals }

// Addr returns the bound address. It is the empty string until the listener
// binds.
func (a *App) Addr() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.ln == nil {
		return ""
	}
	return a.ln.Addr().String()
}

// Run performs the run sequence of the SDD, section 5.3. It returns when the
// context is done, when a signal arrives, or when a fault stops the sequence.
// Call Exit with the result to get the process exit code.
func (a *App) Run(ctx context.Context) error {
	if !a.ran.CompareAndSwap(false, true) {
		return &Fault{
			Stage: StageSetup, Subject: "the application",
			Message: "Run was called a second time",
			Repair:  "Build a new application with New for each Run",
		}
	}
	if err := a.validate(); err != nil {
		return err
	}
	if len(a.signals) > 0 {
		var stop context.CancelFunc
		ctx, stop = signal.NotifyContext(ctx, a.signals...)
		defer stop()
	}

	// Step 2. The boot checks.
	if err := a.runChecks(ctx); err != nil {
		return err
	}
	// Step 3. The migrations.
	if err := a.migrate(ctx); err != nil {
		return err
	}
	// The server listens before the components start, so that /readyz answers
	// 503 while the application comes up.
	serving, err := a.serve()
	if err != nil {
		return err
	}
	// Steps 4 and 5. Construct and start, in registration order.
	started, startErr := a.start(ctx)
	if startErr != nil {
		return errors.Join(startErr, a.shutdown(ctx, started))
	}
	// Step 6. Open the readiness gate.
	a.open.Store(true)
	a.log.InfoContext(ctx, "the application is ready",
		"addr", a.Addr(), "env", a.cfg.Env, "components", len(started))

	// Step 7. Wait for the context, for a signal, or for a server fault.
	select {
	case <-ctx.Done():
	case err := <-serving:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.open.Store(false)
			return errors.Join(&Fault{
				Stage: StageStart, Subject: "the HTTP server",
				Message: "the server stopped while the application ran",
				Repair:  "Read the cause. Repair the server. Start the application again.",
				Err:     err,
			}, a.shutdown(ctx, started))
		}
	}

	// Step 8. Close the readiness gate. The server keeps serving.
	a.open.Store(false)
	a.log.InfoContext(ctx, "the readiness gate is closed", "grace", a.cfg.ShutdownGrace)

	// Steps 9, 10 and 11.
	return a.shutdown(ctx, started)
}

// validate refuses an application that carries a fault a person can repair in
// main.go. It runs before any component starts.
func (a *App) validate() error {
	seen := make(map[string]bool, len(a.components))
	for i, c := range a.components {
		name := c.Name()
		if name == "" {
			return &Fault{
				Stage: StageSetup, Subject: fmt.Sprintf("component %d", i),
				Message: fmt.Sprintf("the component at position %d returns no name", i),
				Repair:  "Return a name from the Name method of that component",
			}
		}
		if seen[name] {
			return &Fault{
				Stage: StageSetup, Subject: name,
				Message: fmt.Sprintf("two components carry the name %q", name),
				Repair:  "Give each component a name that no other component uses",
			}
		}
		seen[name] = true
	}
	for i, c := range a.checks {
		switch {
		case c.Name == "":
			return &Fault{
				Stage: StageSetup, Subject: fmt.Sprintf("check %d", i),
				Message: fmt.Sprintf("the boot check at position %d has an empty Name", i),
				Repair:  "Set the Name field of that boot check to the thing it proves",
			}
		case c.Repair == "":
			return &Fault{
				Stage: StageSetup, Subject: c.Name,
				Message: fmt.Sprintf("the boot check %s has an empty Repair", c.Name),
				Repair:  "Set the Repair field of that boot check to one sentence that says what to do",
			}
		case c.Run == nil:
			return &Fault{
				Stage: StageSetup, Subject: c.Name,
				Message: fmt.Sprintf("the boot check %s has no Run", c.Name),
				Repair:  "Set the Run field of that boot check to the function that proves it",
			}
		}
	}
	return nil
}

// runChecks runs every boot check in registration order. It collects every
// fault, so that one run reports every reason the application cannot start.
func (a *App) runChecks(ctx context.Context) error {
	var faults []*BootFault
	for _, c := range a.checks {
		if err := c.Run(ctx); err != nil {
			faults = append(faults, &BootFault{Check: c.Name, Repair: c.Repair, Err: err})
		}
	}
	if len(faults) == 0 {
		return nil
	}
	return &BootFaults{Faults: faults}
}

// migrate applies the pending migrations when MIGRATE_ON_BOOT is true.
func (a *App) migrate(ctx context.Context) error {
	if !a.cfg.MigrateOnBoot || a.migrator == nil {
		return nil
	}
	a.log.InfoContext(ctx, "applying the pending migrations")
	if err := a.migrator.Migrate(ctx); err != nil {
		return &Fault{
			Stage: StageMigrate, Subject: "the migrator",
			Message: "the migrations did not apply",
			Repair:  "Run `avero migrate status` to see the pending migration, or set MIGRATE_ON_BOOT=false",
			Err:     err,
		}
	}
	return nil
}

// serve binds the listener and serves in a goroutine. The returned channel
// carries the result of Serve.
func (a *App) serve() (<-chan error, error) {
	a.mu.Lock()
	if a.ln == nil {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", a.cfg.Port))
		if err != nil {
			a.mu.Unlock()
			return nil, &Fault{
				Stage: StageStart, Subject: "the HTTP server",
				Message: fmt.Sprintf("the server cannot bind port %d", a.cfg.Port),
				Repair:  "Set PORT to a free port, or stop the process that holds this one",
				Err:     err,
			}
		}
		a.ln = ln
	}
	a.srv = &http.Server{Handler: a.track(a.mux()), ReadHeaderTimeout: 10 * time.Second}
	srv, ln := a.srv, a.ln
	a.mu.Unlock()

	out := make(chan error, 1)
	go func() { out <- srv.Serve(ln) }()
	return out, nil
}

// start calls Start on each component in registration order. It returns the
// components that started. A panic in Start is a fault, so start does not
// recover it.
func (a *App) start(ctx context.Context) ([]Component, error) {
	started := make([]Component, 0, len(a.components))
	for _, c := range a.components {
		a.log.InfoContext(ctx, "starting the component", "component", c.Name())
		if err := c.Start(ctx); err != nil {
			return started, &Fault{
				Stage: StageStart, Subject: c.Name(),
				Message: fmt.Sprintf("the component %s did not start", c.Name()),
				Repair:  fmt.Sprintf("Repair the Start of %s, or remove it from the component list", c.Name()),
				Err:     err,
			}
		}
		started = append(started, c)
	}
	return started, nil
}

// shutdown drains the server and stops the components in reverse order. It
// performs steps 9, 10 and 11 of the run sequence.
//
// One deadline of SHUTDOWN_GRACE covers the whole shutdown, and it starts
// here. The drain and every Stop share it. A shared deadline is the only
// reading under which "Stop receives a context with the SHUTDOWN_GRACE
// deadline" and "exit within SHUTDOWN_GRACE" both hold.
//
// The listener stays open for the whole drain. A probe that arrives after the
// gate closes therefore reads 503 and takes the instance out of rotation. It
// does not read a refused connection.
func (a *App) shutdown(ctx context.Context, started []Component) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.cfg.ShutdownGrace)
	defer cancel()

	a.drain(ctx)

	var errs []error
	for i := len(started) - 1; i >= 0; i-- {
		c := started[i]
		a.log.InfoContext(ctx, "stopping the component", "component", c.Name())
		if err := c.Stop(ctx); err != nil {
			errs = append(errs, &Fault{
				Stage: StageStop, Subject: c.Name(),
				Message: fmt.Sprintf("the component %s did not stop", c.Name()),
				Repair:  fmt.Sprintf("Repair the Stop of %s so that it returns inside SHUTDOWN_GRACE", c.Name()),
				Err:     err,
			})
		}
	}
	return errors.Join(errs...)
}

// drain waits for the in-flight requests to end, then closes the server. It
// warns and returns when the grace expires with a request still in flight.
func (a *App) drain(ctx context.Context) {
	a.mu.RLock()
	srv := a.srv
	a.mu.RUnlock()
	if srv == nil {
		return
	}

	a.draining.Store(true)
	if a.inFlight.Load() == 0 {
		a.markIdle()
	}
	select {
	case <-a.idle:
		if err := srv.Shutdown(ctx); err != nil {
			_ = srv.Close()
		}
	case <-ctx.Done():
		// The grace expired. Drop the connections, so that the process exits
		// inside the grace. The SDD makes this step of the run sequence, so
		// the process still exits 0. See section 5.3, steps 9 and 11. The cut
		// is never silent.
		cut := a.inFlight.Load()
		_ = srv.Close()
		a.log.Warn("the grace expired with requests in flight, so the server cut them",
			"cut", cut, "grace", a.cfg.ShutdownGrace)
	}
}

// track counts the requests inside a handler, so that the drain knows when the
// server is idle.
func (a *App) track(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.inFlight.Add(1)
		defer func() {
			if a.inFlight.Add(-1) == 0 && a.draining.Load() {
				a.markIdle()
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// markIdle reports that no request is in flight. It is safe to call more than
// one time.
func (a *App) markIdle() { a.idleOnce.Do(func() { close(a.idle) }) }

// defaultLogger builds a logger from LOG_LEVEL and LOG_FORMAT.
func defaultLogger(cfg config.BaseConfig) *slog.Logger {
	level := slog.LevelInfo
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	if cfg.LogFormat == "text" {
		return slog.New(slog.NewTextHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, opts))
}
