package main

import (
	"embed"
	"io/fs"

	"github.com/alternayte/avero"
	"github.com/alternayte/avero/assets"
	"github.com/alternayte/drel"

	"board/internal/features/tasks"
)

// dist holds the build of the front end. Vite writes it, and the compiler puts
// it in the binary, so one file holds the server and the front end.
//
//go:embed all:assets/dist
var dist embed.FS

// wire builds the router and the module set.
//
// An inspection command passes a nil engine and a zero configuration, because
// a route registration touches no database and needs no key.
func wire(engine *drel.Engine, cfg Config) (*avero.Wiring, error) {
	r := avero.NewRouter()
	// Stack states the order of the chain one time. API leaves the flash
	// cookie and the CSRF token out, because a JSON client needs neither. A
	// nil engine leaves the transaction out.
	r.Use(avero.Stack{Secret: cfg.Secret, Engine: engine}.API()...)

	modules := avero.Modules(tasks.New(engine))
	if err := modules.Attach(r); err != nil {
		return nil, err
	}

	// The front end owns every path that no route holds. It reads the path
	// itself, so a deep link and a reload reach the same document.
	files, err := fs.Sub(dist, "assets/dist")
	if err != nil {
		return nil, err
	}
	r.Mount("/", assets.SPA(files))

	// Wiring states what this application is. Add a dependency of your own
	// with a lifecycle in Components, and its boot check in Checks.
	return &avero.Wiring{Router: r, Modules: modules}, nil
}
