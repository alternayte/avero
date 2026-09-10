package avero

import "errors"

// Wiring is what wire builds. Serve reads it.
//
// The Router and the Modules are required. The Components and the Checks are
// optional, so an application that adds no dependency of its own states two
// fields.
type Wiring struct {
	// Router holds the routes of the application.
	Router *Router
	// Modules holds the module set. Serve reads the migrations of each
	// module from it.
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
	Checks []Check
}

// validate reports a wiring that Serve cannot use. Each fault states the
// repair. See DX-7.
func (w *Wiring) validate() error {
	switch {
	case w == nil:
		return errors.New("wire returned no wiring: return a *avero.Wiring that holds its Router and its Modules")
	case w.Router == nil:
		return errors.New("the wiring holds no router: set the Router field to the value that avero.NewRouter returned")
	case w.Modules == nil:
		return errors.New("the wiring holds no module set: set the Modules field to the value that avero.Modules returned")
	}
	return nil
}
