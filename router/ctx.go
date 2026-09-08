package router

import (
	"context"
	"net/http"

	"github.com/alternayte/auth-all/store"
	"github.com/alternayte/drel"
)

// Ctx carries one request. A handler reads it and returns a Response. A
// handler never writes to the ResponseWriter, because the transaction commits
// after the handler returns. See the SDD, S4.
type Ctx struct {
	w http.ResponseWriter
	r *http.Request

	// toasts collects the messages that this response shows. The flash
	// middleware carries them across a redirect.
	toasts []Toast
	// wroteHeader guards against a second WriteHeader call.
	wroteHeader bool
}

// newCtx builds a Ctx for one request.
func newCtx(w http.ResponseWriter, r *http.Request) *Ctx { return &Ctx{w: w, r: r} }

// Request returns the request.
func (c *Ctx) Request() *http.Request { return c.r }

// Writer returns the response writer. A handler must not write to it. The
// router writes the Response after the transaction commits.
func (c *Ctx) Writer() http.ResponseWriter { return c.w }

// Context returns the request context. It carries the transaction and the
// session.
func (c *Ctx) Context() context.Context { return c.r.Context() }

// setRequest replaces the request. A middleware that adds a context value
// calls it.
func (c *Ctx) setRequest(r *http.Request) { c.r = r }

// Header returns the response header. A handler sets a header here and the
// Response writes the body.
func (c *Ctx) Header() http.Header { return c.w.Header() }

// WriteHeader sends the status one time. A second call does nothing.
func (c *Ctx) WriteHeader(code int) {
	if c.wroteHeader {
		return
	}
	c.wroteHeader = true
	c.w.WriteHeader(code)
}

// PathValue returns a value from the pattern, such as {id}.
func (c *Ctx) PathValue(name string) string { return c.r.PathValue(name) }

// Tx returns the transaction that the transaction middleware opened. It
// returns nil when no transaction surrounds the request.
//
// The SDD names this method UoW. drel deleted UnitOfWork, because it held no
// connection and could not write an application row and a control row in one
// transaction. The context now carries *drel.Tx.
func (c *Ctx) Tx() *drel.Tx {
	tx, ok := drel.FromContext(c.r.Context())
	if !ok {
		return nil
	}
	return tx
}

// MustTx returns the transaction and panics when none is present. Use it in a
// handler that a transaction must always surround, so that a wiring fault
// fails at once.
func (c *Ctx) MustTx() *drel.Tx { return drel.MustFromContext(c.r.Context()) }

// User returns the signed-in user. It returns nil for an anonymous request.
// auth-all attaches the user with RequireAuth or LoadSession.
func (c *Ctx) User() *store.User { return authUserFrom(c.r.Context()) }

// Session returns the auth-all session. It returns nil for an anonymous
// request.
func (c *Ctx) Session() *store.Session { return authSessionFrom(c.r.Context()) }

// Partial reports whether the client asked for a fragment and not a whole
// page. htmx and Datastar both mark such a request with a header.
//
// A view uses it to render one fragment. internal/ui therefore needs no import
// of either library. See the SDD, S12.
func (c *Ctx) Partial() bool {
	h := c.r.Header
	return h.Get("HX-Request") == "true" || h.Get("Datastar-Request") == "true"
}
