package avero_test

import (
	"context"
	"io"
	"io/fs"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/drel"

	avero "github.com/alternayte/avero"
)

// serveConfig is the configuration of the application under test. It embeds
// BaseConfig, so it carries Base and it needs no method of its own.
type serveConfig struct {
	avero.BaseConfig
}

// serveWire builds a router with one route and no module.
func serveWire(_ *drel.Engine, _ serveConfig) (*avero.Router, *avero.ModuleSet, error) {
	r := avero.NewRouter()
	r.Get("/{$}", func(_ *avero.Ctx) (avero.Response, error) {
		return avero.Text(http.StatusOK, "ready"), nil
	})
	return r, avero.Modules(), nil
}

// An inspection command opens no database and reads no configuration, so
// `avero routes` runs on a machine with no database. See DX-8.
func TestServeInspects(t *testing.T) {
	var out strings.Builder
	code := avero.Serve(avero.Service[serveConfig]{
		Args: []string{avero.InspectPrefix + "routes"},
		Wire: serveWire,
		Out:  &out,
		Err:  io.Discard,
	})
	if code != 0 {
		t.Fatalf("the inspection returned the code %d", code)
	}
	if !strings.Contains(out.String(), "/") {
		t.Fatalf("the route table is empty: %q", out.String())
	}
}

// A configuration fault stops the process with the code 1 before it serves.
func TestServeStopsOnAConfigurationFault(t *testing.T) {
	t.Setenv("AVERO_SECRET", "")
	var errOut strings.Builder
	code := avero.Serve(avero.Service[serveConfig]{
		Args: nil,
		Wire: serveWire,
		Out:  io.Discard,
		Err:  &errOut,
	})
	if code != 1 {
		t.Fatalf("the run returned the code %d", code)
	}
	if !strings.Contains(errOut.String(), "AVERO_SECRET") {
		t.Fatalf("the fault does not name the variable: %q", errOut.String())
	}
}

// A database address that Serve cannot open stops the process with the code
// 1 before it serves, and the fault names the variable that DSNEnv states.
func TestServeStopsOnADatabaseFault(t *testing.T) {
	t.Setenv("AVERO_SECRET", strings.Repeat("a", 64))
	var errOut strings.Builder
	code := avero.Serve(avero.Service[serveConfig]{
		Args:   nil,
		Wire:   serveWire,
		DSN:    func(serveConfig) string { return "postgres://bad:%zz@nope/db" },
		DSNEnv: "APP_DATABASE_URL",
		Out:    io.Discard,
		Err:    &errOut,
	})
	if code != 1 {
		t.Fatalf("the run returned the code %d", code)
	}
	if !strings.Contains(errOut.String(), "the database does not open") {
		t.Fatalf("the fault does not state the repair: %q", errOut.String())
	}
	if !strings.Contains(errOut.String(), "APP_DATABASE_URL") {
		t.Fatalf("the fault does not name the variable: %q", errOut.String())
	}
}

// Serve opens a database, runs the migrator and the migration check, and
// stops on the end of the context.
func TestServeRunsWithADatabaseAndStops(t *testing.T) {
	t.Setenv("AVERO_SECRET", strings.Repeat("a", 64))
	t.Setenv("MIGRATE_ON_BOOT", "true")
	dsn := filepath.Join(t.TempDir(), "serve.db")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned an error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() {
		done <- avero.Serve(avero.Service[serveConfig]{
			Args:       nil,
			Wire:       serveWire,
			DSN:        func(serveConfig) string { return dsn },
			Migrations: func() []fs.FS { return []fs.FS{} },
			Ctx:        ctx,
			Out:        io.Discard,
			Err:        io.Discard,
			Options:    []avero.Option{avero.WithoutSignals(), avero.WithListener(ln)},
		})
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("the run returned the code %d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the run did not stop")
	}
}

// Serve registers MigrationCheckOnFS when MIGRATE_ON_BOOT is false, and the
// check passes on a database with no pending migration.
func TestServeRunsWithAMigrationCheckAndStops(t *testing.T) {
	t.Setenv("AVERO_SECRET", strings.Repeat("a", 64))
	dsn := filepath.Join(t.TempDir(), "serve.db")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned an error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() {
		done <- avero.Serve(avero.Service[serveConfig]{
			Args:       nil,
			Wire:       serveWire,
			DSN:        func(serveConfig) string { return dsn },
			Migrations: func() []fs.FS { return []fs.FS{} },
			Ctx:        ctx,
			Out:        io.Discard,
			Err:        io.Discard,
			Options:    []avero.Option{avero.WithoutSignals(), avero.WithListener(ln)},
		})
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("the run returned the code %d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the run did not stop")
	}
}

// Serve starts the application, serves the handler and stops on the end of
// the context.
func TestServeRunsAndStops(t *testing.T) {
	t.Setenv("AVERO_SECRET", strings.Repeat("a", 64))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned an error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() {
		done <- avero.Serve(avero.Service[serveConfig]{
			Args:    nil,
			Wire:    serveWire,
			Ctx:     ctx,
			Out:     io.Discard,
			Err:     io.Discard,
			Options: []avero.Option{avero.WithoutSignals(), avero.WithListener(ln)},
		})
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("the run returned the code %d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the run did not stop")
	}
}

// flagComponent sets a flag when it starts, so a test proves that a
// component built from the module set reaches the application.
type flagComponent struct {
	started *bool
}

func (flagComponent) Name() string { return "flag" }

func (c flagComponent) Start(context.Context) error {
	*c.started = true
	return nil
}

func (flagComponent) Stop(context.Context) error { return nil }

// Serve registers the components that Components builds from the module set
// that Wire returns, so a component started by the application sets its
// flag.
func TestServeRegistersComponentsFromTheModuleSet(t *testing.T) {
	t.Setenv("AVERO_SECRET", strings.Repeat("a", 64))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned an error: %v", err)
	}

	var started bool
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() {
		done <- avero.Serve(avero.Service[serveConfig]{
			Args: nil,
			Wire: serveWire,
			Components: func(*avero.ModuleSet) []avero.Component {
				return []avero.Component{flagComponent{started: &started}}
			},
			Ctx:     ctx,
			Out:     io.Discard,
			Err:     io.Discard,
			Options: []avero.Option{avero.WithoutSignals(), avero.WithListener(ln)},
		})
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("the run returned the code %d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the run did not stop")
	}
	if !started {
		t.Fatal("the component did not start")
	}
}
