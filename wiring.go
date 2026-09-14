package avero

import (
	"errors"
	"net/http"
)

// Wiring is what wire builds. Serve reads it.
//
// An application states its routes in one of two ways. It states the Router,
// which is the whole host: the typed routes, the module system and the
// inspection commands. Or it states the Handler, which is any http.Handler,
// such as a chi router or a mux of the standard library. An application that
// states the Handler keeps the lifecycle, the configuration, the boot checks
// and `avero doctor`, and it keeps its own router.
//
// The Modules, the Components and the Checks are optional, so a simple
// application states one field.
type Wiring struct {
	// Router holds the routes of the application. State it or state the
	// Handler, and not both.
	Router *Router
	// Handler serves the requests of an application that holds a router of
	// another library. State it or state the Router, and not both.
	//
	// The inspection commands read the routes of an Avero router, so
	// `avero routes` and `avero openapi` report nothing for such an
	// application. `avero doctor` reads the configuration and the checks, so
	// it works for both.
	Handler http.Handler
	// Modules holds the module set. Serve reads the migrations of each
	// module from it. An application that states no module holds no module
	// set, and the field stays nil.
	Modules *ModuleSet
	// Components are the parts of the application that hold a lifecycle,
	// such as an emailer, a blob store or a cache. Serve starts them in this
	// order and stops them in reverse order.
	//
	// A constructor must not dial, connect or read a file, because wire also
	// runs for an inspection command with a nil engine and a zero
	// configuration. Build the value in the constructor. Connect in Start.
	Components []Component
	// Checks are the boot checks of the application. Serve runs them before
	// the process serves, and `avero doctor` reports them. See DX-8.
	//
	// A check runs before every component starts. A check that depends on a
	// component must open its own connection and close it, and must not
	// assume the connection that Start of that component opened.
	Checks []Check
}

// validate reports a wiring that Serve cannot use. Each fault states the
// repair. See DX-7.
func (w *Wiring) validate() error {
	switch {
	case w == nil:
		return errors.New("wire returned no wiring: return a *avero.Wiring that holds its Router or its Handler")
	case w.Router == nil && w.Handler == nil:
		return errors.New("the wiring holds no routes: set the Router field to the value that avero.NewRouter returned, or set the Handler field to an http.Handler of your own")
	case w.Router != nil && w.Handler != nil:
		return errors.New("the wiring holds a Router and a Handler: state one of the two, because Serve can mount one handler only")
	}
	return nil
}

// handler returns the http.Handler of the application.
//
// The router reports every registration fault here, before the process serves.
// See DX-8.
func (w *Wiring) handler() (http.Handler, error) {
	if w.Handler != nil {
		return w.Handler, nil
	}
	return w.Router.Handler()
}
