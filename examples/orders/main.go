// Command orders is an Avero application of the api shape.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/alternayte/avero"
	"github.com/alternayte/drel"
)

func main() { os.Exit(run()) }

// run builds the application and returns the exit code of the process.
func run() int {
	// An inspection command reads the routes and the modules of this
	// application. It opens no database and it reads no configuration, so
	// `avero routes` works on a machine with no database.
	if avero.Inspecting(os.Args[1:]) {
		r, modules, err := wire(nil, Config{})
		if err != nil {
			return avero.Exit(os.Stderr, err)
		}
		return avero.Inspect[Config](os.Args[1:], os.Stdout, os.Stderr, r, modules,
			checks(os.Getenv("DATABASE_URL"))...)
	}

	ctx := context.Background()
	cfg, err := avero.Load[Config](ctx)
	if err != nil {
		return avero.Exit(os.Stderr, err)
	}

	engine, err := drel.NewEngine(cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "the database does not open: %v\n  → Prove DATABASE_URL, and start the database\n", err)
		return 1
	}
	defer engine.Close()

	r, modules, err := wire(engine, *cfg)
	if err != nil {
		return avero.Exit(os.Stderr, err)
	}
	handler, err := r.Handler()
	if err != nil {
		return avero.Exit(os.Stderr, err)
	}
	_ = modules

	// The boot checks read the engine that the application already opened,
	// so a start opens one connection pool and no more.
	boot := []avero.Check{avero.SecretCheck(cfg.Secret), avero.DatabaseCheckOn(engine)}
	if !cfg.MigrateOnBoot {
		boot = append(boot, avero.MigrationCheckOnFS(engine, migrationSets()...))
	}

	app := avero.New(cfg.BaseConfig,
		avero.WithHandler(handler),
		avero.WithChecks(boot...),
		avero.WithMigrator(migrator{engine: engine}))

	return avero.Exit(os.Stderr, app.Run(ctx))
}

// checks are the boot checks that `avero doctor` runs. The doctor holds no
// engine, so the check opens its own connection and closes it. The boot uses
// the engine of the application instead. See DX-8.
func checks(dsn string) []avero.Check {
	if dsn == "" {
		return nil
	}
	return []avero.Check{
		avero.DatabaseCheck(dsn),
		avero.MigrationCheckFS(dsn, migrationSets()...),
	}
}

// migrator applies the pending migrations when MIGRATE_ON_BOOT is true.
type migrator struct {
	engine *drel.Engine
}

// Migrate applies every migration that the database does not hold.
//
// Each feature slice embeds its own migrations, and drel merges the sets in
// version order. The files come from the binary, so one artifact carries the
// server and the schema.
func (m migrator) Migrate(ctx context.Context) error {
	_, err := m.engine.ApplyMigrationsFS(ctx, migrationSets()...)
	return err
}
