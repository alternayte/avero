package avero

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/url"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/alternayte/avero/config"
	"github.com/alternayte/avero/host"
	"github.com/alternayte/avero/internal/migrations"
	"github.com/alternayte/avero/module"
	"github.com/alternayte/avero/openapi"
	"github.com/alternayte/drel"
)

// InspectPrefix marks a command that the avero binary sends to the application.
// The CLI runs `go run . avero:routes`, because the routes and the modules of
// an application live in its own code. See the SDD, S14.
const InspectPrefix = "avero:"

// Inspecting reports an inspection command. The application calls it before it
// loads the configuration and before it opens the database, so an inspection
// needs neither.
//
//	func main() {
//	    r, modules := wire(nil)
//	    if avero.Inspecting(os.Args[1:]) {
//	        os.Exit(avero.Inspect[Config](os.Args[1:], os.Stdout, os.Stderr, r, modules))
//	    }
//	    os.Exit(run())
//	}
func Inspecting(args []string) bool {
	return len(args) > 0 && strings.HasPrefix(args[0], InspectPrefix)
}

// Inspect answers one inspection command and returns the exit code.
//
// The type parameter is the configuration of the application, so avero:doctor
// reports every variable that the application reads. The checks are the boot
// checks, so the doctor answers the question that DX-8 asks before the process
// starts.
func Inspect[T any](args []string, out, errOut io.Writer, r *Router, set *ModuleSet, checks ...Check) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(errOut, "avero: the inspection command is absent")
		return 1
	}
	name := strings.TrimPrefix(args[0], InspectPrefix)
	asJSON := false
	for _, a := range args[1:] {
		if a == "--json" || a == "-json" {
			asJSON = true
			continue
		}
		_, _ = fmt.Fprintf(errOut, "avero %s: the flag %q is not known\n  → Run the command with --json or with no flag\n", name, a)
		return 1
	}

	switch name {
	case "routes":
		rep, err := r.Report()
		if err != nil {
			_, _ = fmt.Fprintln(errOut, err)
			return 1
		}
		return write(out, errOut, rep, rep.String(), asJSON)
	case "modules":
		rep, err := set.Report()
		if err != nil {
			_, _ = fmt.Fprintln(errOut, err)
			return 1
		}
		return write(out, errOut, rep, rep.String(), asJSON)
	case "openapi":
		rep, err := r.Report()
		if err != nil {
			_, _ = fmt.Fprintln(errOut, err)
			return 1
		}
		// The application names itself, so the description carries the name
		// that a person reads. The version of the description follows the
		// binary.
		doc := openapi.Describe(AppName(), AppVersion(), rep.Routes, r.API())
		_, _ = io.WriteString(out, doc.String())
		return 0
	case "schema":
		rep := Schema(set)
		return write(out, errOut, rep, rep.String(), asJSON)
	case "doctor":
		_, report, err := config.LoadFrom[T](context.Background(), config.Loader{})
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		doc := host.Doctor(ctx, report, err, checks)
		if writeErr := doc.Write(out, asJSON); writeErr != nil {
			_, _ = fmt.Fprintln(errOut, writeErr)
			return 1
		}
		if !doc.OK {
			return 1
		}
		return 0
	default:
		_, _ = fmt.Fprintf(errOut, "avero: the inspection command %q is not known\n  → Run routes, modules, openapi, schema or doctor\n", name)
		return 1
	}
}

// write prints one report as a table or as JSON.
func write(out, errOut io.Writer, doc any, table string, asJSON bool) int {
	if !asJSON {
		_, _ = io.WriteString(out, table)
		return 0
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		_, _ = fmt.Fprintln(errOut, err)
		return 1
	}
	return 0
}

// The schema report. See the module package for the description of a model.
type (
	// SchemaReport lists the models of every module. `avero schema` prints
	// it.
	SchemaReport = module.SchemaReport
)

// Schema returns the models that the modules describe.
func Schema(set *ModuleSet) *SchemaReport { return module.Schema(set) }

// MigrationCheck proves that the database holds every migration of the
// directory. It opens its own connection, so `avero doctor` runs it with no
// application. An application that already holds an engine passes it to
// MigrationCheckOn, which opens nothing.
//
// Register it with WithChecks, so a pending migration stops the process before
// it serves. See DX-8.
func MigrationCheck(dsn, dir string) Check {
	check := MigrationCheckOn(nil, dir)
	check.Run = func(ctx context.Context) error {
		e, err := drel.NewEngine(dsn)
		if err != nil {
			return fmt.Errorf("the database does not open: %w", err)
		}
		defer e.Close()
		return migrationsPending(ctx, e, dir)
	}
	return check
}

// MigrationCheckOnFS proves the migrations of the embedded sets of the feature
// slices. Each slice embeds its own, and drel merges them in version order.
func MigrationCheckOnFS(e *drel.Engine, sets ...fs.FS) Check {
	return Check{
		Name:   "the pending migrations",
		Repair: "Run `avero migrate up`, or set MIGRATE_ON_BOOT=true",
		Run: func(ctx context.Context) error {
			if e == nil {
				return fmt.Errorf("the check holds no database")
			}
			pending, err := migrations.PendingFS(ctx, e, sets...)
			if err != nil {
				return err
			}
			return pendingFault(pending)
		},
	}
}

// MigrationCheckFS proves the embedded migration sets against a database that
// the check opens itself. `avero doctor` holds no engine, so it uses this one.
func MigrationCheckFS(dsn string, sets ...fs.FS) Check {
	check := MigrationCheckOnFS(nil, sets...)
	check.Run = func(ctx context.Context) error {
		e, err := drel.NewEngine(dsn)
		if err != nil {
			return fmt.Errorf("the database does not open: %w", err)
		}
		defer e.Close()
		pending, pendErr := migrations.PendingFS(ctx, e, sets...)
		if pendErr != nil {
			return pendErr
		}
		return pendingFault(pending)
	}
	return check
}

// MigrationCheckOn proves the migrations with an engine that the application
// already opened.
func MigrationCheckOn(e *drel.Engine, dir string) Check {
	return Check{
		Name:   "the pending migrations",
		Repair: "Run `avero migrate up`, or set MIGRATE_ON_BOOT=true",
		Run: func(ctx context.Context) error {
			if e == nil {
				return fmt.Errorf("the check holds no database")
			}
			return migrationsPending(ctx, e, dir)
		},
	}
}

// migrationsPending returns a fault that names each pending migration.
func migrationsPending(ctx context.Context, e *drel.Engine, dir string) error {
	pending, err := migrations.Pending(ctx, e, dir)
	if err != nil {
		return err
	}
	return pendingFault(pending)
}

// pendingFault names each pending migration, or returns nil.
func pendingFault(pending []migrations.Migration) error {
	if len(pending) == 0 {
		return nil
	}
	names := make([]string, 0, len(pending))
	for _, m := range pending {
		names = append(names, m.Version+"_"+m.Name)
	}
	sort.Strings(names)
	return fmt.Errorf("%d migrations are pending: %s", len(pending), strings.Join(names, ", "))
}

// UnpackMigrations writes the migrations of an embedded file system into a
// temporary directory and returns it with the function that removes it.
//
// An application that embeds its migrations therefore applies them from its
// own binary, and one artifact holds the server, the front end and the schema.
//
//	//go:embed all:migrations
//	var migrations embed.FS
//
//	dir, clean, err := avero.UnpackMigrations(migrations, "migrations")
//	defer clean()
func UnpackMigrations(fsys fs.FS, root string) (string, func(), error) {
	return migrations.Unpack(fsys, root)
}

// DatabaseCheck proves that the database answers. It opens its own connection,
// so `avero doctor` runs it with no application. An application that already
// holds an engine passes it to DatabaseCheckOn.
func DatabaseCheck(dsn string) Check {
	return Check{
		Name:   "the database",
		Repair: "Start the database. Prove DATABASE_URL.",
		Run: func(ctx context.Context) error {
			e, err := drel.NewEngine(dsn)
			if err != nil {
				return fmt.Errorf("the database does not open: %w", err)
			}
			defer e.Close()
			return answers(ctx, e)
		},
	}
}

// DatabaseCheckOn proves the database with an engine that the application
// already opened.
func DatabaseCheckOn(e *drel.Engine) Check {
	return Check{
		Name:   "the database",
		Repair: "Start the database. Prove DATABASE_URL.",
		Run: func(ctx context.Context) error {
			if e == nil {
				return fmt.Errorf("the check holds no database")
			}
			return answers(ctx, e)
		},
	}
}

// answers proves that the database reads one row.
func answers(ctx context.Context, e *drel.Engine) error {
	if _, err := e.Exec(ctx, "SELECT 1"); err != nil {
		return fmt.Errorf("the database does not answer: %w", err)
	}
	return nil
}

// BrokerCheck proves that the broker accepts a connection. It opens a TCP
// connection to the host of the address, because the broker library belongs to
// the application.
func BrokerCheck(address string) Check {
	return Check{
		Name:   "the broker",
		Repair: "Start the broker. Prove AMQP_URL.",
		Run: func(ctx context.Context) error {
			u, err := url.Parse(address)
			if err != nil || u.Host == "" {
				return fmt.Errorf("the broker address %q is not a URL", address)
			}
			host := u.Host
			if u.Port() == "" {
				host = net.JoinHostPort(u.Hostname(), defaultPort(u.Scheme))
			}
			var d net.Dialer
			conn, err := d.DialContext(ctx, "tcp", host)
			if err != nil {
				return fmt.Errorf("the broker at %s does not answer: %w", host, err)
			}
			return conn.Close()
		},
	}
}

// defaultPort returns the port of a broker scheme.
func defaultPort(scheme string) string {
	switch scheme {
	case "amqps":
		return "5671"
	case "amqp":
		return "5672"
	default:
		return "5672"
	}
}

// AppName returns the name of the application, which the description of the
// API carries.
//
// The binary reads its own module path, so a person needs to state the name in
// no second place.
func AppName() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Path == "" {
		return "application"
	}
	path := info.Main.Path
	if i := strings.LastIndex(path, "/"); i >= 0 && i+1 < len(path) {
		return path[i+1:]
	}
	return path
}

// AppVersion returns the version of the application, which the description of
// the API carries. A build from the source names 0.0.0.
func AppVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || strings.Contains(info.Main.Version, "devel") {
		return "0.0.0"
	}
	return strings.TrimPrefix(info.Main.Version, "v")
}
