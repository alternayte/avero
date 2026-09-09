package router

import (
	"log/slog"

	"github.com/alternayte/avero/config"
	"github.com/alternayte/drel"
)

// Stack builds the middleware chain of an application in the order that the
// SDD states, S4: request ID, recover, access log, trace, flash, session,
// CSRF, transaction, authentication.
//
// The order is a correctness property. The flash middleware must read the
// cookie before a handler adds a toast. The transaction middleware must sit
// inside the session, so a rollback leaves no signed-in user without a row.
// A hand-written chain puts that order in every main.go, and a wrong order
// fails silently. Stack states it one time.
//
// A zero field leaves its middleware out, so an application that runs no
// database and no authentication passes the secret alone.
//
//	r.Use(router.Stack{
//	    Secret: cfg.Secret,
//	    Logger: log,
//	    Trace:  provider.HTTPMiddleware(),
//	    Engine: engine,
//	}.Middleware()...)
//
// The router package must not import telemetry or auth-all, because both
// import the router. Trace, Session and Auth therefore carry a value that the
// application builds.
type Stack struct {
	// Secret signs the CSRF cookie and the flash cookie. It is mandatory.
	Secret config.Secret
	// Logger writes the access log. A nil value leaves the access log out.
	Logger *slog.Logger
	// Trace is the middleware of the telemetry provider. A zero value leaves
	// the trace out.
	Trace Middleware
	// Session holds the session middleware of auth-all, adapted with Adapt.
	Session []Middleware
	// Engine opens the transaction. A nil value leaves the transaction out,
	// which an inspection command needs.
	Engine *drel.Engine
	// Auth holds the authentication middleware of auth-all, adapted with
	// Adapt. It runs last, so it reads the session and the transaction.
	Auth []Middleware
	// Cookie carries the options of the two cookies that Avero owns.
	Cookie []CookieOption
}

// Middleware returns the chain of an application that serves a browser. It
// holds the flash cookie and the CSRF token, so a form posts safely and a
// toast crosses a redirect. The first member is the outermost one.
func (s Stack) Middleware() []Middleware { return s.build(true) }

// API returns the chain of an application that answers JSON. It holds the same
// members in the same order, and it leaves out the two cookies that a browser
// form needs. A client that carries a token needs no CSRF token, and a client
// that reads JSON shows no toast.
//
// The two methods make the choice a name and not a flag, so a shape that needs
// the CSRF token cannot lose it by a forgotten field.
func (s Stack) API() []Middleware { return s.build(false) }

// build assembles the chain. cookies states whether the two cookies of a
// browser belong in it.
func (s Stack) build(cookies bool) []Middleware {
	mws := []Middleware{RequestID(), Recover(s.Logger)}
	if s.Logger != nil {
		mws = append(mws, AccessLog(s.Logger))
	}
	if s.Trace.Wrap != nil {
		mws = append(mws, s.Trace)
	}
	if cookies {
		mws = append(mws, Flash(s.Secret, s.Cookie...))
	}
	mws = append(mws, s.Session...)
	if cookies {
		mws = append(mws, CSRF(s.Secret, s.Cookie...))
	}
	if s.Engine != nil {
		mws = append(mws, Transaction(s.Engine))
	}
	return append(mws, s.Auth...)
}
