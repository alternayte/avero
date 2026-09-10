// Command board is an Avero application of the spa shape.
package main

import (
	"os"

	"github.com/alternayte/avero"
)

// main starts the application and returns the exit code of the process.
//
// Serve runs the sequence of the host. It answers an inspection command,
// loads the configuration, and opens the database. It builds the router and
// registers the boot checks and the migrator, then serves until a signal.
// Read the doc comment of avero.Serve for the long form.
func main() {
	os.Exit(avero.Serve(avero.Service[Config]{
		Args:       os.Args[1:],
		Wire:       wire,
		DSN:        func(c Config) string { return c.DatabaseURL },
		Migrations: migrationSets,
	}))
}
