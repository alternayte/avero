package cli

import (
	"context"
	"fmt"

	"github.com/alternayte/avero/internal/inspect"
)

// runRoutes prints the routes of the application.
func runRoutes(ctx context.Context, s Streams, args []string) int {
	return ask(ctx, s, inspect.Routes, args)
}

// runModules prints the contribution of each module.
func runModules(ctx context.Context, s Streams, args []string) int {
	return ask(ctx, s, inspect.Modules, args)
}

// runSchema prints the models that the modules describe.
func runSchema(ctx context.Context, s Streams, args []string) int {
	return ask(ctx, s, inspect.Schema, args)
}

// runDoctor proves the configuration, the database and the broker.
func runDoctor(ctx context.Context, s Streams, args []string) int {
	return ask(ctx, s, inspect.Doctor, args)
}

// ask runs one inspection command of the application and writes its answer.
func ask(ctx context.Context, s Streams, name string, args []string) int {
	asJSON := false
	for _, a := range args {
		switch a {
		case "--json", "-json":
			asJSON = true
		default:
			return failf(s, "avero %s: the flag %q is not known\n  → Run the command with --json or with no flag", name, a)
		}
	}
	result, err := inspect.Run(ctx, dirOf(s), name, asJSON)
	if err != nil {
		return fail(s, err)
	}
	_, _ = fmt.Fprint(s.Out, result.Stdout)
	if result.Stderr != "" {
		_, _ = fmt.Fprintln(s.Err, result.Stderr)
	}
	return result.Code
}
