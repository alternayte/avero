package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alternayte/avero/internal/inspect"
	"github.com/alternayte/avero/openapi"
)

// runRoutes prints the routes of the application, or its OpenAPI description.
func runRoutes(ctx context.Context, s Streams, args []string) int {
	for i, a := range args {
		if a == "--openapi" || a == "-openapi" {
			return runOpenAPI(ctx, s, append(append([]string(nil), args[:i]...), args[i+1:]...))
		}
	}
	return ask(ctx, s, inspect.Routes, args)
}

// runOpenAPI writes the OpenAPI description of the application.
//
// The application states its own routes, so the command runs it with the
// inspection flag. A route registration touches no database and opens no
// port, so the command needs no infrastructure. See the SDD, S13.
func runOpenAPI(ctx context.Context, s Streams, args []string) int {
	out := ""
	server := ""
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--out" || a == "-out":
			i++
			if i >= len(args) {
				return failf(s, "avero routes: --out names no file\n  → Write --out openapi.json")
			}
			out = args[i]
		case strings.HasPrefix(a, "--out="):
			out = strings.TrimPrefix(a, "--out=")
		case a == "--server" || a == "-server":
			i++
			if i >= len(args) {
				return failf(s, "avero routes: --server names no address\n  → Write --server https://api.example.com")
			}
			server = args[i]
		case strings.HasPrefix(a, "--server="):
			server = strings.TrimPrefix(a, "--server=")
		case a == "--json" || a == "-json":
			// The description is JSON, so the flag changes nothing.
		default:
			return failf(s, "avero routes: the flag %q is not known\n  → Run `avero routes --openapi [--out openapi.json] [--server https://api.example.com]`", a)
		}
	}

	dir := dirOf(s)
	result, err := inspect.Run(ctx, dir, inspect.OpenAPI, false)
	if err != nil {
		return fail(s, err)
	}
	if result.Code != 0 {
		if result.Stderr != "" {
			_, _ = fmt.Fprintln(s.Err, result.Stderr)
		}
		return result.Code
	}
	doc, err := openapi.Parse([]byte(result.Stdout))
	if err != nil {
		return fail(s, err)
	}
	if server != "" {
		doc.Servers = []openapi.Server{{URL: server}}
	}
	body := doc.String() + "\n"
	if out == "" {
		_, _ = fmt.Fprint(s.Out, body)
		return 0
	}
	if err := os.WriteFile(filepath.Join(dir, out), []byte(body), 0o644); err != nil {
		return failf(s, "avero routes: %s does not write\n  → Give the process the right to write the application directory", out)
	}
	_, _ = fmt.Fprintf(s.Out, "%s: %d paths\n", out, len(doc.Paths))
	return 0
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
