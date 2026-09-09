package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	avero "github.com/alternayte/avero"
)

// runRoutes prints the routes of the application.
func runRoutes(ctx context.Context, s Streams, args []string) int {
	return inspect(ctx, s, "routes", args)
}

// runModules prints the contribution of each module.
func runModules(ctx context.Context, s Streams, args []string) int {
	return inspect(ctx, s, "modules", args)
}

// runSchema prints the models that the modules describe.
func runSchema(ctx context.Context, s Streams, args []string) int {
	return inspect(ctx, s, "schema", args)
}

// runDoctor proves the configuration, the database and the broker.
func runDoctor(ctx context.Context, s Streams, args []string) int {
	return inspect(ctx, s, "doctor", args)
}

// inspect runs the application with an inspection command.
//
// The routes, the modules and the checks of an application live in its own
// code, so the CLI asks the application. The application answers before it
// loads the configuration and before it opens the database, so the command
// works on a machine with no database. See the SDD, S14.
func inspect(ctx context.Context, s Streams, name string, args []string) int {
	asJSON := false
	for _, a := range args {
		switch a {
		case "--json", "-json":
			asJSON = true
		default:
			return failf(s, "avero %s: the flag %q is not known\n  → Run the command with --json or with no flag", name, a)
		}
	}
	dir := dirOf(s)
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		return failf(s, "avero %s: this directory holds no Go module\n  → Run the command in the directory of an Avero application, or write one with `avero new`", name)
	}
	run := []string{"run", ".", avero.InspectPrefix + name}
	if asJSON {
		run = append(run, "--json")
	}
	cmd := exec.CommandContext(ctx, "go", run...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	_, _ = s.Out.Write(out.Bytes())
	if text := strings.TrimSpace(errOut.String()); text != "" {
		_, _ = fmt.Fprintln(s.Err, text)
	}
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if ok := asExit(err, &exit); ok {
		return exit.ExitCode()
	}
	return failf(s, "avero %s: the application did not run: %v\n  → Run `go build ./...` and repair the fault that the compiler names", name, err)
}
