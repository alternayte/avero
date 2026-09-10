package main

import (
	"net/http"

	"github.com/alternayte/avero"
	"github.com/alternayte/drel"

	"orders/internal/features/posts"
)

// wire builds the wiring of the application: the router, the module set,
// the components and the boot checks.
//
// An inspection command passes a nil engine. `avero doctor` passes the
// loaded configuration, and every other inspection command passes a zero
// configuration, because a route registration touches no database and needs
// no key.
func wire(engine *drel.Engine, cfg Config) (*avero.Wiring, error) {
	r := avero.NewRouter()
	// Stack states the order of the chain one time. API leaves the flash
	// cookie and the CSRF token out, because a JSON client needs neither. A
	// nil engine leaves the transaction out.
	r.Use(avero.Stack{Secret: cfg.Secret, Engine: engine}.API()...)

	// The service answers JSON, so the root path states the name of the
	// service.
	r.Get("/{$}", func(c *avero.Ctx) (avero.Response, error) {
		return avero.JSON(http.StatusOK, map[string]string{"service": "orders"}), nil
	})

	modules := avero.Modules(posts.New(engine))
	if err := modules.Attach(r); err != nil {
		return nil, err
	}
	// Wiring states what this application is. Add a dependency of your own
	// with a lifecycle in Components, and its boot check in Checks.
	return &avero.Wiring{Router: r, Modules: modules}, nil
}
