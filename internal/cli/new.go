package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alternayte/avero/assets"
	"github.com/alternayte/avero/codegen"
	"github.com/alternayte/avero/codegen/client"
	"github.com/alternayte/avero/scaffold"
)

// runNew writes a new application.
func runNew(ctx context.Context, s Streams, args []string) int {
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
	// The application must build at once, so the command reads the
	// dependencies and writes every generated file.
	if out, err := goCommand(ctx, opts.Dir, "mod", "tidy"); err != nil {
		return failf(s, "avero new: the dependencies do not read\n%s\n  → Prove the network, then run `go mod tidy` in %s",
			strings.TrimSpace(out), name)
	}
	if err := Templ(ctx, opts.Dir); err != nil {
		return fail(s, err)
	}
	if err := Vendor(ctx, opts.Dir, opts.Shape, s); err != nil {
		return fail(s, err)
	}
	if _, err := codegen.Generate(opts.Dir); err != nil {
		return fail(s, err)
	}
	if _, err := client.Generate(opts.Dir); err != nil {
		return fail(s, err)
	}
	for _, path := range written {
		_, _ = fmt.Fprintln(s.Out, path)
	}
	_, _ = fmt.Fprintf(s.Out, "\nThe application %s is ready.\n\n\tcd %s\n\tcp .env.example .env\n\tavero migrate up\n\tavero dev\n", name, name)
	return 0
}

// FrontEnd holds the modules that the spa shape vendors. `avero js pin`
// fetches each one as a bundled ES module and records its address and its hash
// in avero.lock, so a later build needs no network and no Node.js. See DX-9.
var FrontEnd = []struct{ Name, URL string }{
	{"react", "https://esm.sh/react@19.2.0/es2022/react.bundle.mjs"},
	{"react/jsx-runtime", "https://esm.sh/react@19.2.0/es2022/jsx-runtime.bundle.mjs"},
	{"react-dom/client", "https://esm.sh/react-dom@19.2.0/es2022/client.bundle.mjs"},
	{"@tanstack/react-query", "https://esm.sh/@tanstack/react-query@5.90.2/es2022/react-query.bundle.mjs"},
}

// Vendor fetches the front end modules of a shape. Only the spa shape carries
// one.
func Vendor(ctx context.Context, dir, shape string, s Streams) error {
	if shape != scaffold.ShapeSPA {
		return nil
	}
	for _, module := range FrontEnd {
		if err := assets.PinJS(ctx, assets.PinConfig{Dir: dir}, module.Name, module.URL); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(s.Out, "pinned %s\n", module.Name)
	}
	return nil
}
