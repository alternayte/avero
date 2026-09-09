// Package inspect runs an inspection command of an application and returns the
// JSON document that it printed.
//
// The routes, the modules and the models of an application live in its own
// code, so the CLI and the MCP server both ask the application. The
// application answers before it loads the configuration and before it opens
// the database. See the SDD, S14 and S16.
package inspect

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Prefix marks a command that the binary sends to the application.
const Prefix = "avero:"

// The inspection commands.
const (
	// Routes returns the routes of the application.
	Routes = "routes"
	// Modules returns the contribution of each module.
	Modules = "modules"
	// Schema returns the models that the modules describe.
	Schema = "schema"
	// Doctor proves the configuration, the database and the broker.
	Doctor = "doctor"
)

// Result is the answer of one inspection.
type Result struct {
	// Stdout holds the document that the application printed.
	Stdout string
	// Stderr holds the faults that the application printed.
	Stderr string
	// Code is the exit code of the application.
	Code int
}

// Run asks the application for one inspection. asJSON returns the document
// that the schema of the report accepts.
func Run(ctx context.Context, dir, command string, asJSON bool) (*Result, error) {
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		return nil, fmt.Errorf("avero %s: this directory holds no Go module\n  → Run the command in the directory of an Avero application, or write one with `avero new`",
			command)
	}
	args := []string{"run", ".", Prefix + command}
	if asJSON {
		args = append(args, "--json")
	}
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()

	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	result := &Result{Stdout: out.String(), Stderr: strings.TrimSpace(errOut.String())}
	if err == nil {
		return result, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		result.Code = exit.ExitCode()
		return result, nil
	}
	return nil, fmt.Errorf("avero %s: the application did not run: %w\n  → Run `go build ./...` and repair the fault that the compiler names",
		command, err)
}
