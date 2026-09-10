package avero

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"

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
	// Wire builds the router and the module set. An inspection command
	// passes a nil engine and a zero configuration.
	Wire func(engine *drel.Engine, cfg C) (*Router, *ModuleSet, error)
	// DSN reads the database address from the configuration. A nil value
	// starts an application with no database.
	DSN func(cfg C) string
	// DSNEnv names the variable that `avero doctor` reads for the database
	// address. An empty value reads DATABASE_URL.
	DSNEnv string
	// Migrations holds the migration files of each feature slice. drel
	// merges the sets in version order. A nil value applies no migration.
	Migrations func() []fs.FS
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
// It runs the sequence of section 5.3 of the SDD: it answers an inspection
// command, it loads the configuration, it opens the database, it builds the
// router, it registers the boot checks and the migrator, and it runs the
// application until a signal or the end of the context.
//
// An application with an unusual start calls Load, New and Run itself. Serve
// adds no behaviour that those three do not hold.
//
//	func main() {
//	    os.Exit(avero.Serve(avero.Service[Config]{
//	        Args:       os.Args[1:],
//	        Wire:       wire,
//	        DSN:        func(c Config) string { return c.DatabaseURL },
//	        Migrations: migrationSets,
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

	// An inspection command reads the routes and the modules. It opens no
	// database and it reads no configuration, so `avero routes` works on a
	// machine with no database.
	if Inspecting(s.Args) {
		var zero C
		r, modules, err := s.Wire(nil, zero)
		if err != nil {
			return Exit(errOut, err)
		}
		return Inspect[C](s.Args, out, errOut, r, modules, s.doctorChecks()...)
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
				"the database does not open: %v\n  → Prove %s, and start the database\n",
				err, s.dsnEnv())
			return 1
		}
		defer engine.Close()
	}

	r, modules, err := s.Wire(engine, *cfg)
	if err != nil {
		return Exit(errOut, err)
	}
	handler, err := r.Handler()
	if err != nil {
		return Exit(errOut, err)
	}
	_ = modules

	// The boot checks read the engine that the application already opened,
	// so a start opens one connection pool and no more. See DX-8.
	checks := []Check{SecretCheck(base.Secret)}
	if engine != nil {
		checks = append(checks, DatabaseCheckOn(engine))
		if !base.MigrateOnBoot && s.Migrations != nil {
			checks = append(checks, MigrationCheckOnFS(engine, s.Migrations()...))
		}
	}

	opts := []Option{WithHandler(handler), WithChecks(checks...)}
	if engine != nil && s.Migrations != nil {
		opts = append(opts, WithMigrator(fsMigrator{engine: engine, sets: s.Migrations}))
	}
	opts = append(opts, s.Options...)

	return Exit(errOut, New(base, opts...).Run(ctx))
}

// dsnEnv returns the name of the variable that holds the database address.
func (s Service[C]) dsnEnv() string {
	if s.DSNEnv == "" {
		return "DATABASE_URL"
	}
	return s.DSNEnv
}

// doctorChecks returns the boot checks of `avero doctor`.
//
// The doctor holds no engine, so a check opens its own connection and closes
// it. The boot uses the engine of the application instead. See DX-8.
func (s Service[C]) doctorChecks() []Check {
	dsn := os.Getenv(s.dsnEnv())
	if dsn == "" || s.DSN == nil {
		return nil
	}
	checks := []Check{DatabaseCheck(dsn)}
	if s.Migrations != nil {
		checks = append(checks, MigrationCheckFS(dsn, s.Migrations()...))
	}
	return checks
}

// fsMigrator applies the pending migrations when MIGRATE_ON_BOOT is true.
//
// Each feature slice embeds its own migrations, and drel merges the sets in
// version order. The files come from the binary, so one artifact carries the
// server and the schema.
type fsMigrator struct {
	engine *drel.Engine
	sets   func() []fs.FS
}

// Migrate applies every migration that the database does not hold.
func (m fsMigrator) Migrate(ctx context.Context) error {
	_, err := m.engine.ApplyMigrationsFS(ctx, m.sets()...)
	return err
}
