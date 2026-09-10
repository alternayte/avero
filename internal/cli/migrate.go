package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/alternayte/avero/internal/migrations"
	"github.com/alternayte/drel"
)

// runMigrate writes and applies the migrations of the application.
//
// An application that states its models in drel.yaml lets drel write and apply
// them, because drel reads the models and writes the difference. An
// application that holds SQL files of its own keeps the reader of Avero.
func runMigrate(ctx context.Context, s Streams, args []string) int {
	if len(args) == 0 {
		return failf(s, "avero migrate: the form is `avero migrate new <name>|up|down|status`\n  → Name the step that you want")
	}
	dir := dirOf(s)
	if _, err := os.Stat(filepath.Join(dir, DrelConfig)); err == nil {
		return drelMigrate(ctx, s, dir, args)
	}
	project, err := LoadProject(dir)
	if err != nil {
		return fail(s, err)
	}
	migrations := filepath.Join(dir, project.Migrations)

	switch args[0] {
	case "new":
		if len(args) < 2 {
			return failf(s, "avero migrate new: the form is `avero migrate new <name>`\n  → Name the migration, such as `avero migrate new add_posts`")
		}
		return fail(s, newMigration(s, migrations, strings.Join(args[1:], "_")))
	case "up":
		return withEngine(ctx, s, func(e *drel.Engine) error {
			n, err := e.ApplyMigrations(ctx, migrations)
			if err != nil {
				return fmt.Errorf("avero migrate up: the migrations did not apply: %w%s", err,
					hint("Repair the SQL that the message names. Run the command again."))
			}
			_, _ = fmt.Fprintf(s.Out, "applied %d migrations\n", n)
			return nil
		})
	case "status":
		return withEngine(ctx, s, func(e *drel.Engine) error { return status(ctx, s, e, migrations) })
	case "down":
		return withEngine(ctx, s, func(e *drel.Engine) error { return down(ctx, s, e, migrations) })
	default:
		return failf(s, "avero migrate: the step %q is not known\n  → Write new, up, down or status", args[0])
	}
}

// drelMigrate runs the migration command of drel.
//
// `avero migrate new <name>` reads the models and writes the difference, so a
// person writes no SQL by hand. The other steps read and apply the sets of
// every feature slice.
func drelMigrate(ctx context.Context, s Streams, dir string, args []string) int {
	step := args[0]
	switch step {
	case "new", "up", "down", "status", "lint", "check":
	default:
		return failf(s, "avero migrate: the step %q is not known\n  → Write new, up, down, status, lint or check", step)
	}
	if step == "new" && len(args) < 2 {
		return failf(s, "avero migrate new: the form is `avero migrate new <name>`\n  → Name the migration, such as `avero migrate new add_posts`")
	}

	command := append([]string{"tool", "drel", "migrate"}, args...)
	cmd := exec.CommandContext(ctx, "go", command...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	cmd.Stdout = s.Out
	cmd.Stderr = s.Err
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		return failf(s, "avero migrate: drel did not run: %v\n  → Run `go mod tidy`. Run the command again.", err)
	}
	return 0
}

// withEngine opens the database of DATABASE_URL and runs fn.
func withEngine(_ context.Context, s Streams, fn func(e *drel.Engine) error) int {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return failf(s, "avero migrate: DATABASE_URL is absent\n  → Set DATABASE_URL in .env, or export it in the shell")
	}
	e, err := drel.NewEngine(dsn)
	if err != nil {
		return failf(s, "avero migrate: the database does not open: %v\n  → Prove DATABASE_URL. Start the database.", err)
	}
	defer e.Close()
	return fail(s, fn(e))
}

// newMigration writes the up file and the down file of one migration.
func newMigration(s Streams, dir, name string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("avero migrate new: %s does not open\n  → Give the process the right to write the migrations directory", dir)
	}
	slug := strings.ReplaceAll(strings.ToLower(name), " ", "_")
	version := time.Now().UTC().Format("20060102150405")
	for exists(dir, version) {
		version = next(version)
	}
	up := filepath.Join(dir, version+"_"+slug+".up.sql")
	down := filepath.Join(dir, version+"_"+slug+".down.sql")
	if err := os.WriteFile(up, []byte("-- Write the change here.\n"), 0o644); err != nil {
		return fmt.Errorf("avero migrate new: %s does not write\n  → Give the process the right to write the migrations directory", up)
	}
	if err := os.WriteFile(down, []byte("-- Write the reverse of the change here.\n"), 0o644); err != nil {
		return fmt.Errorf("avero migrate new: %s does not write\n  → Give the process the right to write the migrations directory", down)
	}
	_, _ = fmt.Fprintln(s.Out, up)
	_, _ = fmt.Fprintln(s.Out, down)
	return nil
}

// exists reports a version that the directory already holds.
func exists(dir, version string) bool {
	matches, _ := filepath.Glob(filepath.Join(dir, version+"_*.up.sql"))
	return len(matches) > 0
}

// next returns the version that follows this one.
func next(version string) string {
	var n int64
	_, _ = fmt.Sscanf(version, "%d", &n)
	return fmt.Sprintf("%014d", n+1)
}

// status prints one row for each migration.
func status(ctx context.Context, s Streams, e *drel.Engine, dir string) error {
	files, err := migrations.Read(dir)
	if err != nil {
		return err
	}
	done := migrations.Applied(ctx, e)
	w := tabwriter.NewWriter(s.Out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "VERSION\tNAME\tSTATE\tAPPLIED AT")
	pending := 0
	for _, m := range files {
		state := "pending"
		at := "-"
		if when, ok := done[m.Version]; ok {
			state, at = "applied", when
		} else {
			pending++
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.Version, m.Name, state, at)
	}
	_ = w.Flush()
	if pending > 0 {
		_, _ = fmt.Fprintf(s.Out, "\n%d migrations are pending. Run `avero migrate up`.\n", pending)
	}
	return nil
}

// down reverses the last applied migration.
func down(ctx context.Context, s Streams, e *drel.Engine, dir string) error {
	files, err := migrations.Read(dir)
	if err != nil {
		return err
	}
	done := migrations.Applied(ctx, e)
	for i := len(files) - 1; i >= 0; i-- {
		m := files[i]
		if _, ok := done[m.Version]; !ok {
			continue
		}
		body, readErr := os.ReadFile(migrations.File(dir, m, "down"))
		if readErr != nil {
			return fmt.Errorf("avero migrate down: the down file of %s is absent%s", m.Version,
				hint(fmt.Sprintf("Write %s_%s.down.sql. Run the command again.", m.Version, m.Name)))
		}
		err := e.WithTx(ctx, func(ctx context.Context) error {
			tx := drel.MustFromContext(ctx)
			if _, err := tx.Exec(ctx, string(body)); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, "DELETE FROM drel_migrations WHERE version = $1", m.Version)
			return err
		})
		if err != nil {
			return fmt.Errorf("avero migrate down: %s did not reverse: %w%s", m.Version, err,
				hint("Repair the SQL of the down file. Run the command again."))
		}
		_, _ = fmt.Fprintf(s.Out, "reversed %s_%s\n", m.Version, m.Name)
		return nil
	}
	_, _ = fmt.Fprintln(s.Out, "no migration is applied")
	return nil
}
