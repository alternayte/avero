package avero_test

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/alternayte/drel"

	avero "github.com/alternayte/avero"
)

// serveConfig is the configuration of the application under test. It embeds
// BaseConfig, so it carries Base and it needs no method of its own.
type serveConfig struct {
	avero.BaseConfig
}

// serveWire builds a wiring with one route and no module.
func serveWire(_ *drel.Engine, _ serveConfig) (*avero.Wiring, error) {
	r := avero.NewRouter()
	r.Get("/{$}", func(_ *avero.Ctx) (avero.Response, error) {
		return avero.Text(http.StatusOK, "ready"), nil
	})
	return &avero.Wiring{Router: r, Modules: avero.Modules()}, nil
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
// 1 before it serves, and the fault states the repair.
func TestServeStopsOnADatabaseFault(t *testing.T) {
	t.Setenv("AVERO_SECRET", strings.Repeat("a", 64))
	var errOut strings.Builder
	code := avero.Serve(avero.Service[serveConfig]{
		Args: nil,
		Wire: serveWire,
		DSN:  func(serveConfig) string { return "postgres://bad:%zz@nope/db" },
		Out:  io.Discard,
		Err:  &errOut,
	})
	if code != 1 {
		t.Fatalf("the run returned the code %d", code)
	}
	if !strings.Contains(errOut.String(), "the database does not open") {
		t.Fatalf("the fault does not state the repair: %q", errOut.String())
	}
	if !strings.Contains(errOut.String(), "the database address") {
		t.Fatalf("the fault does not state the repair: %q", errOut.String())
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
			Args:    nil,
			Wire:    serveWire,
			DSN:     func(serveConfig) string { return dsn },
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
			Args:    nil,
			Wire:    serveWire,
			DSN:     func(serveConfig) string { return dsn },
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

// A wire that returns a wiring with no router stops the run and names the
// repair.
func TestServeStopsOnAWiringFault(t *testing.T) {
	t.Setenv("AVERO_SECRET", strings.Repeat("a", 64))
	var errOut strings.Builder
	code := avero.Serve(avero.Service[serveConfig]{
		Wire: func(*drel.Engine, serveConfig) (*avero.Wiring, error) {
			return &avero.Wiring{Modules: avero.Modules()}, nil
		},
		Out: io.Discard,
		Err: &errOut,
	})
	if code != 1 {
		t.Fatalf("the run returned the code %d", code)
	}
	if !strings.Contains(errOut.String(), "Router") {
		t.Fatalf("the fault does not name the field: %q", errOut.String())
	}
}

// recorder is a component that records its own lifecycle. A test proves the
// order of a start and of a stop.
type recorder struct {
	name string
	log  *[]string
}

func (r recorder) Name() string { return r.name }

func (r recorder) Start(context.Context) error {
	*r.log = append(*r.log, "start "+r.name)
	return nil
}

func (r recorder) Stop(context.Context) error {
	*r.log = append(*r.log, "stop "+r.name)
	return nil
}

// A component that wire builds starts with the application and stops with it.
// Avero starts in registration order and stops in reverse order.
func TestServeStartsTheComponentsOfTheWiring(t *testing.T) {
	t.Setenv("AVERO_SECRET", strings.Repeat("a", 64))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("the listener does not open: %v", err)
	}

	var log []string
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() {
		done <- avero.Serve(avero.Service[serveConfig]{
			Wire: func(*drel.Engine, serveConfig) (*avero.Wiring, error) {
				r := avero.NewRouter()
				r.Get("/{$}", func(_ *avero.Ctx) (avero.Response, error) {
					return avero.Text(http.StatusOK, "ready"), nil
				})
				return &avero.Wiring{
					Router:  r,
					Modules: avero.Modules(),
					Components: []avero.Component{
						recorder{name: "first", log: &log},
						recorder{name: "second", log: &log},
					},
				}, nil
			},
			Ctx:     ctx,
			Out:     io.Discard,
			Err:     io.Discard,
			Options: []avero.Option{avero.WithListener(listener), avero.WithoutSignals()},
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

	want := []string{"start first", "start second", "stop second", "stop first"}
	if !slices.Equal(log, want) {
		t.Fatalf("the lifecycle reads %v and it must read %v", log, want)
	}
}

// A check that the wiring carries stops a bad boot with the code 1, and the
// fault names the repair. See DX-8.
func TestServeRunsTheChecksOfTheWiring(t *testing.T) {
	t.Setenv("AVERO_SECRET", strings.Repeat("a", 64))
	var errOut strings.Builder
	code := avero.Serve(avero.Service[serveConfig]{
		Wire: func(*drel.Engine, serveConfig) (*avero.Wiring, error) {
			return &avero.Wiring{
				Router:  avero.NewRouter(),
				Modules: avero.Modules(),
				Checks: []avero.Check{{
					Name:   "the mail server",
					Repair: "Set SMTP_URL to the address of the mail server",
					Run: func(context.Context) error {
						return errors.New("the mail server does not answer")
					},
				}},
			}, nil
		},
		Out: io.Discard,
		Err: &errOut,
	})
	if code != 1 {
		t.Fatalf("the run returned the code %d", code)
	}
	if !strings.Contains(errOut.String(), "SMTP_URL") {
		t.Fatalf("the fault does not state the repair: %q", errOut.String())
	}
}

// `avero doctor` reports a check that the wiring carries, so a person proves
// a dependency of the application before the process serves. See DX-8.
func TestTheDoctorReportsTheChecksOfTheWiring(t *testing.T) {
	t.Setenv("AVERO_SECRET", strings.Repeat("a", 64))
	var out strings.Builder
	code := avero.Serve(avero.Service[serveConfig]{
		Args: []string{avero.InspectPrefix + "doctor"},
		Wire: func(*drel.Engine, serveConfig) (*avero.Wiring, error) {
			return &avero.Wiring{
				Router:  avero.NewRouter(),
				Modules: avero.Modules(),
				Checks: []avero.Check{{
					Name:   "the mail server",
					Repair: "Set SMTP_URL to the address of the mail server",
					Run:    func(context.Context) error { return nil },
				}},
			}, nil
		},
		Out: &out,
		Err: io.Discard,
	})
	if code != 0 {
		t.Fatalf("the doctor returned the code %d", code)
	}
	if !strings.Contains(out.String(), "the mail server") {
		t.Fatalf("the doctor does not name the check: %q", out.String())
	}
}

// migrating is a module that carries migration files, so Serve reads the set
// from the module set and not from a second list.
type migrating struct{ files fs.FS }

func (migrating) Name() string { return "migrating" }

func (m migrating) Migrations() fs.FS { return m.files }

// Serve applies the migrations that a module states, so an application keeps
// one list and not two.
func TestServeAppliesTheMigrationsOfAModule(t *testing.T) {
	t.Setenv("AVERO_SECRET", strings.Repeat("a", 64))
	t.Setenv("MIGRATE_ON_BOOT", "true")
	dsn := "file:" + filepath.Join(t.TempDir(), "migrate.db")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("the listener does not open: %v", err)
	}

	files := fstest.MapFS{
		"0001_widgets.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE widgets (id TEXT PRIMARY KEY);"),
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() {
		done <- avero.Serve(avero.Service[serveConfig]{
			Wire: func(*drel.Engine, serveConfig) (*avero.Wiring, error) {
				return &avero.Wiring{
					Router:  avero.NewRouter(),
					Modules: avero.Modules(migrating{files: files}),
				}, nil
			},
			DSN:     func(serveConfig) string { return dsn },
			Ctx:     ctx,
			Out:     io.Discard,
			Err:     io.Discard,
			Options: []avero.Option{avero.WithListener(listener), avero.WithoutSignals()},
		})
	}()

	time.Sleep(500 * time.Millisecond)
	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("the run returned the code %d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the run did not stop")
	}

	// No migration is pending, so the migrator read the set of the module
	// and applied it. MigrationCheckOnFS states the same fact that the boot
	// check states, so the test needs no raw query.
	engine, err := drel.NewEngine(dsn)
	if err != nil {
		t.Fatalf("the database does not open: %v", err)
	}
	defer engine.Close()
	check := avero.MigrationCheckOnFS(engine, files)
	if err := check.Run(context.Background()); err != nil {
		t.Fatalf("a migration is still pending: %v", err)
	}
}

// The doctor reads the address from the configuration and not from the
// process environment, so an application that composes its address gets a
// real database check.
func TestTheDoctorReadsTheAddressFromTheConfiguration(t *testing.T) {
	t.Setenv("AVERO_SECRET", strings.Repeat("a", 64))
	t.Setenv("DATABASE_URL", "")
	var out strings.Builder
	code := avero.Serve(avero.Service[serveConfig]{
		Args: []string{avero.InspectPrefix + "doctor"},
		Wire: serveWire,
		DSN: func(serveConfig) string {
			return "file:" + filepath.Join(t.TempDir(), "doctor.db")
		},
		Out: &out,
		Err: io.Discard,
	})
	if code != 0 {
		t.Fatalf("the doctor returned the code %d", code)
	}
	if !strings.Contains(out.String(), "database") {
		t.Fatalf("the doctor states no database check: %q", out.String())
	}
}

// databaseConfig carries a variable of its own, so a test can prove that the
// doctor reads it from the configuration and not from a fixed string.
type databaseConfig struct {
	avero.BaseConfig
	DatabaseURL string `env:"DATABASE_URL"`
}

func databaseWire(_ *drel.Engine, _ databaseConfig) (*avero.Wiring, error) {
	return &avero.Wiring{Router: avero.NewRouter(), Modules: avero.Modules()}, nil
}

// The doctor exists for a broken configuration. An absent secret must not
// hide the database check, because the report must prove every part of the
// configuration at the same time, and the database check reads the address
// that the loader read, not a zero value. See DX-8.
func TestTheDoctorReportsTheDatabaseWithAnAbsentSecret(t *testing.T) {
	t.Setenv("AVERO_SECRET", "")
	t.Setenv("DATABASE_URL", "file:"+filepath.Join(t.TempDir(), "doctor.db"))
	var out strings.Builder
	code := avero.Serve(avero.Service[databaseConfig]{
		Args: []string{avero.InspectPrefix + "doctor"},
		Wire: databaseWire,
		DSN:  func(c databaseConfig) string { return c.DatabaseURL },
		Out:  &out,
		Err:  io.Discard,
	})
	if code != 1 {
		t.Fatalf("the doctor returned the code %d, want 1", code)
	}
	if !strings.Contains(out.String(), "AVERO_SECRET") {
		t.Fatalf("the doctor states no absent secret: %q", out.String())
	}
	if !strings.Contains(out.String(), "database") {
		t.Fatalf("the doctor states no database check: %q", out.String())
	}
}

// A short secret loads without a fault, but it fails the boot. The doctor
// must report that failure by name, not just the presence of the variable.
func TestTheDoctorReportsAShortSecret(t *testing.T) {
	t.Setenv("AVERO_SECRET", "too-short")
	var out strings.Builder
	code := avero.Serve(avero.Service[serveConfig]{
		Args: []string{avero.InspectPrefix + "doctor"},
		Wire: serveWire,
		Out:  &out,
		Err:  io.Discard,
	})
	if code != 1 {
		t.Fatalf("the doctor returned the code %d, want 1", code)
	}
	if !strings.Contains(out.String(), "the length of AVERO_SECRET") {
		t.Fatalf("the doctor states no failed length check: %q", out.String())
	}
}

// An inspection command that is not the doctor reads no configuration, so
// `avero routes` works on a machine with no database. See DX-8.
func TestTheRoutesCommandReadsNoConfiguration(t *testing.T) {
	t.Setenv("AVERO_SECRET", "")
	var out strings.Builder
	code := avero.Serve(avero.Service[serveConfig]{
		Args: []string{avero.InspectPrefix + "routes"},
		Wire: serveWire,
		DSN:  func(serveConfig) string { return "file:/does/not/exist/x.db" },
		Out:  &out,
		Err:  io.Discard,
	})
	if code != 0 {
		t.Fatalf("the routes command returned the code %d", code)
	}
}
