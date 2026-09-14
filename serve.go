package avero

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/alternayte/avero/internal/migrations"
	"github.com/alternayte/drel"
)

// Configurer gives Serve the base configuration of an application.
//
// BaseConfig carries the method, so a configuration that embeds it satisfies
// the constraint and writes no code.
type Configurer interface {
	Base() BaseConfig
}

// Service states the parts of one application that Serve needs.
//
// The zero value of every optional field states a sensible default, so a
// simple application fills three fields. See the SDD, 5.3, for the sequence
// that Serve runs.
type Service[C Configurer] struct {
	// Args holds the command line without the name of the program. Pass
	// os.Args[1:].
	Args []string
	// Wire builds the wiring of the application. An inspection command
	// passes a nil engine, and every command but `avero doctor` passes a
	// zero configuration.
	Wire func(engine *drel.Engine, cfg C) (*Wiring, error)
	// DSN reads the database address from the configuration. A nil value
	// starts an application with no database.
	DSN func(cfg C) string
	// Options pass to New after the options that Serve builds, so an
	// application can add a component or a check of its own.
	Options []Option
	// Ctx is the context of the run. A nil value uses context.Background.
	Ctx context.Context
	// Out and Err receive the output. A nil value uses os.Stdout and
	// os.Stderr.
	Out, Err io.Writer
}

// Serve runs one Avero application and returns the exit code of the process.
//
// It runs the sequence of section 5.3 of the SDD. It answers an inspection
// command, loads the configuration, and opens the database. It builds the
// router and registers the boot checks and the migrator. It then runs the
// application until a signal or the end of the context.
//
// An application with an unusual start calls Load, New and Run itself. Serve
// adds no behaviour that those three do not hold.
//
//	func main() {
//	    os.Exit(avero.Serve(avero.Service[Config]{
//	        Args: os.Args[1:],
//	        Wire: wire,
//	        DSN:  func(c Config) string { return c.DatabaseURL },
//	    }))
//	}
func Serve[C Configurer](s Service[C]) int {
	out, errOut := s.Out, s.Err
	if out == nil {
		out = os.Stdout
	}
	if errOut == nil {
		errOut = os.Stderr
	}
	ctx := s.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// An inspection command reads the routes and the modules. Every command
	// but `avero doctor` opens no database and reads no configuration, so
	// `avero routes` works on a machine with no database.
	if Inspecting(s.Args) {
		if step, ok := migrateStep(s.Args); ok {
			return s.migrate(ctx, step, out, errOut)
		}
		return s.inspect(ctx, out, errOut)
	}

	cfg, err := Load[C](ctx)
	if err != nil {
		return Exit(errOut, err)
	}
	base := (*cfg).Base()

	var engine *drel.Engine
	if s.DSN != nil {
		engine, err = drel.NewEngine(s.DSN(*cfg))
		if err != nil {
			_, _ = fmt.Fprintf(errOut,
				"the database does not open: %v\n  → Prove the database address in the configuration. Start the database.\n",
				err)
			return 1
		}
		defer engine.Close()
	}

	w, err := s.Wire(engine, *cfg)
	if err != nil {
		return Exit(errOut, err)
	}
	if err := w.validate(); err != nil {
		return Exit(errOut, err)
	}
	handler, err := w.handler()
	if err != nil {
		return Exit(errOut, err)
	}

	// A module states its own migration files. The module set merges them
	// in registration order, so an application keeps one list and not two.
	// An application that states no module set holds no such file.
	sets := w.Modules.Migrations()

	// The boot checks read the engine that the application already opened,
	// so a start opens one connection pool and no more. See DX-8.
	checks := []Check{SecretCheck(base.Secret)}
	if engine != nil {
		checks = append(checks, DatabaseCheckOn(engine))
		if !base.MigrateOnBoot && len(sets) > 0 {
			checks = append(checks, MigrationCheckOnFS(engine, sets...))
		}
	}
	// The checks of the application run after the checks of Avero, so the
	// secret, the database and the migrations are proved first.
	checks = append(checks, w.Checks...)

	opts := []Option{WithHandler(handler), WithChecks(checks...)}
	if engine != nil && len(sets) > 0 {
		opts = append(opts, WithMigrator(fsMigrator{engine: engine, sets: sets}))
	}
	if len(w.Components) > 0 {
		opts = append(opts, WithComponents(w.Components...))
	}
	opts = append(opts, s.Options...)

	return Exit(errOut, New(base, opts...).Run(ctx))
}

// inspect answers one inspection command.
//
// `avero doctor` reports the checks of the application, so it loads the
// configuration. Every other command reads none, so `avero routes` works on a
// machine with no database. See DX-8.
//
// wire receives a nil engine in both cases. A route registration touches no
// database, and a boot check opens its own connection.
func (s Service[C]) inspect(ctx context.Context, out, errOut io.Writer) int {
	var cfg C
	doctor := isDoctor(s.Args)
	if doctor {
		// A configuration that does not load does not stop the doctor.
		// Inspect reports the fault itself. LoadFrom returns the value it
		// already filled beside the fault, so the doctor proves the part
		// of the configuration that did load, such as the database
		// address, together with the variable that did not.
		if loaded, _, _ := LoadFrom[C](ctx, Loader{}); loaded != nil {
			cfg = *loaded
		}
	}

	w, err := s.Wire(nil, cfg)
	if err != nil {
		return Exit(errOut, err)
	}
	if err := w.validate(); err != nil {
		return Exit(errOut, err)
	}

	var checks []Check
	if doctor {
		checks = s.doctorChecks(cfg, w)
	}
	return Inspect[C](s.Args, out, errOut, w.Router, w.Modules, checks...)
}

// NoModuleMigrations is the exit code of `avero:migrate` for an application
// whose modules state no migration.
//
// The CLI reads it and applies the files of the migrations directory that
// avero.json names, so an application that holds SQL of its own keeps the
// reader of Avero.
const NoModuleMigrations = 3

// MigrateCommand is the inspection command that applies the migrations of the
// modules of an application, or reports the migrations that are pending.
//
// The migrations of a module stand in the binary of the application, so the
// application applies them and the CLI asks for it. `avero migrate up` runs
// `go run . avero:migrate up`.
const MigrateCommand = "migrate"

// migrateStep returns the step of the migration command: up or status.
func migrateStep(args []string) (string, bool) {
	if len(args) == 0 || strings.TrimPrefix(args[0], InspectPrefix) != MigrateCommand {
		return "", false
	}
	if len(args) > 1 {
		return args[1], true
	}
	return "up", true
}

// migrate applies the migrations of every module, or reports the ones that the
// database does not hold.
//
// The boot applies the same sets when MIGRATE_ON_BOOT is true, and
// MigrationCheckOnFS proves the same sets before the process serves. One
// source therefore answers the command, the boot and the check, and a module
// of a library never stands outside it.
//
// The command reads the configuration with LoadFrom, which returns the part
// that did load beside a fault. A migration needs the address of the database
// and nothing else, so a script migrates with DATABASE_URL alone and needs no
// key and no whole configuration.
func (s Service[C]) migrate(ctx context.Context, step string, out, errOut io.Writer) int {
	switch step {
	case "up", "status":
	default:
		_, _ = fmt.Fprintf(errOut, "avero migrate: the step %q is not known\n  → Write up or status\n", step)
		return 1
	}
	var cfg C
	if loaded, _, _ := LoadFrom[C](ctx, Loader{}); loaded != nil {
		cfg = *loaded
	}
	if s.DSN == nil {
		_, _ = fmt.Fprintln(errOut, "avero migrate: this application states no database\n  → Set the DSN field of avero.Service, which reads the address from the configuration")
		return 1
	}
	dsn := s.DSN(cfg)
	if dsn == "" {
		_, _ = fmt.Fprintln(errOut, "avero migrate: the configuration names no database\n  → Set DATABASE_URL in .env, or export it in the shell")
		return 1
	}

	// A route registration and a module constructor touch no database, so the
	// wiring runs with no engine. See the Components field of Wiring.
	w, err := s.Wire(nil, cfg)
	if err != nil {
		return Exit(errOut, err)
	}
	if err := w.validate(); err != nil {
		return Exit(errOut, err)
	}
	sets := w.Modules.Migrations()
	if len(sets) == 0 {
		// The application keeps its migrations in a directory of its own, so
		// the CLI reads that directory. The code states the case, because an
		// exit of zero would state that the command applied every migration.
		_, _ = fmt.Fprintln(errOut, "avero migrate: no module of this application states a migration")
		return NoModuleMigrations
	}

	engine, err := drel.NewEngine(dsn)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "avero migrate: the database does not open: %v\n  → Prove the database address in the configuration. Start the database.\n", err)
		return 1
	}
	defer engine.Close()

	if step == "status" {
		pending, err := migrations.PendingFS(ctx, engine, sets...)
		if err != nil {
			return Exit(errOut, err)
		}
		if len(pending) == 0 {
			_, _ = fmt.Fprintln(out, "every migration is applied")
			return 0
		}
		for _, m := range pending {
			_, _ = fmt.Fprintf(out, "pending %s %s\n", m.Version, m.Name)
		}
		return 1
	}

	n, err := engine.ApplyMigrationsFS(ctx, sets...)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "avero migrate: the migrations did not apply: %v\n  → Repair the SQL that the message names. Run the command again.\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(out, "applied %d migrations\n", n)
	return 0
}

// isDoctor reports the doctor command.
func isDoctor(args []string) bool {
	return len(args) > 0 && strings.TrimPrefix(args[0], InspectPrefix) == "doctor"
}

// doctorChecks returns the boot checks that `avero doctor` runs.
//
// The doctor holds no engine. SecretCheck reads no database. DatabaseCheck
// and MigrationCheckFS each open their own connection and close it. w.Checks
// must do the same, by the rule that the Checks field of Wiring states. The
// boot uses the engine of the application instead. See DX-8.
func (s Service[C]) doctorChecks(cfg C, w *Wiring) []Check {
	checks := []Check{SecretCheck(cfg.Base().Secret)}
	if s.DSN == nil {
		return append(checks, w.Checks...)
	}
	dsn := s.DSN(cfg)
	if dsn == "" {
		return append(checks, w.Checks...)
	}
	checks = append(checks, DatabaseCheck(dsn))
	if sets := w.Modules.Migrations(); len(sets) > 0 {
		checks = append(checks, MigrationCheckFS(dsn, sets...))
	}
	return append(checks, w.Checks...)
}

// fsMigrator applies the pending migrations when MIGRATE_ON_BOOT is true.
//
// Each feature slice carries its own migrations, and drel merges the sets in
// version order. The files come from the binary, so one artifact carries the
// server and the schema.
type fsMigrator struct {
	engine *drel.Engine
	sets   []fs.FS
}

// Migrate applies every migration that the database does not hold.
func (m fsMigrator) Migrate(ctx context.Context) error {
	_, err := m.engine.ApplyMigrationsFS(ctx, m.sets...)
	return err
}
