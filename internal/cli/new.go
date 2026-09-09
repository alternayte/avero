package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alternayte/avero/codegen"
	"github.com/alternayte/avero/codegen/client"
	"github.com/alternayte/avero/scaffold"
)

// runNew writes a new application.
func runNew(_ context.Context, s Streams, args []string) int {
	opts := scaffold.Options{}
	var name string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--shape" || a == "-shape":
			i++
			if i >= len(args) {
				return failf(s, "avero new: --shape names no shape\n  → Write --shape ssr, --shape spa or --shape api")
			}
			opts.Shape = args[i]
		case strings.HasPrefix(a, "--shape="):
			opts.Shape = strings.TrimPrefix(a, "--shape=")
		case a == "--hypermedia" || a == "-hypermedia":
			i++
			if i >= len(args) {
				return failf(s, "avero new: --hypermedia names no adapter\n  → Write --hypermedia datastar or --hypermedia htmx")
			}
			opts.Hypermedia = args[i]
		case strings.HasPrefix(a, "--hypermedia="):
			opts.Hypermedia = strings.TrimPrefix(a, "--hypermedia=")
		case a == "--module" || a == "-module":
			i++
			if i >= len(args) {
				return failf(s, "avero new: --module names no path\n  → Write --module github.com/you/blog")
			}
			opts.Module = args[i]
		case strings.HasPrefix(a, "--module="):
			opts.Module = strings.TrimPrefix(a, "--module=")
		case a == "--replace" || a == "-replace":
			i++
			if i >= len(args) {
				return failf(s, "avero new: --replace names no directory\n  → Write --replace ../avero, which points the module at a local copy of the host")
			}
			opts.Replace = args[i]
		case strings.HasPrefix(a, "--replace="):
			opts.Replace = strings.TrimPrefix(a, "--replace=")
		case strings.HasPrefix(a, "-"):
			return failf(s, "avero new: the flag %q is not known\n  → Run `avero help new` to see the form of the command", a)
		default:
			if name != "" {
				return failf(s, "avero new: the command takes one name\n  → Run `avero new <name>`, such as `avero new blog`")
			}
			name = a
		}
	}
	if name == "" {
		return failf(s, "avero new: the application has no name\n  → Run `avero new <name>`, such as `avero new blog`")
	}
	opts.Name = name
	opts.Dir = filepath.Join(dirOf(s), name)

	written, err := scaffold.Write(opts)
	if err != nil {
		return fail(s, err)
	}
	// The generated binding of each input type must exist before the first
	// build, so the application compiles at once.
	if _, err := codegen.Generate(opts.Dir); err != nil {
		return fail(s, err)
	}
	if _, err := client.Generate(opts.Dir); err != nil {
		return fail(s, err)
	}
	for _, path := range written {
		_, _ = fmt.Fprintln(s.Out, path)
	}
	_, _ = fmt.Fprintf(s.Out, "\nThe application %s is ready.\n\n\tcd %s\n\tcp .env.example .env\n\tavero migrate up\n\tgo run .\n", name, name)
	return 0
}
