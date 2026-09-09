package router

import (
	"context"
	"net/http"
	"time"

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
	// onInvalid answers a validation fault. The router supplies it.
	onInvalid func(c *Ctx, f *Fields) Response
	// status is the status that Status named. A zero value means that the
	// method of the route decides. See Status.
	status int
}

// Status names the status of this one answer.
//
// A handler that returns a value needs it for the case that the method does
// not state. A GET answers 200 and a POST answers 201, so a handler calls this
// for 202 or for 200 after a POST.
//
//	func (m *Module) Create(c *avero.Ctx, in CreateInput) (View, error) {
//	    c.Status(http.StatusAccepted)
//	    return view(row), nil
//	}
func (c *Ctx) Status(code int) { c.status = code }

// StatusOf returns the status that Status named, or zero.
func (c *Ctx) StatusOf() int { return c.status }

// newCtx builds a Ctx for one request.
func newCtx(w http.ResponseWriter, r *http.Request) *Ctx { return &Ctx{w: w, r: r} }

// NewCtx builds a Ctx for one request. A test calls a handler with it and
// needs no router. The router builds its own.
func NewCtx(w http.ResponseWriter, r *http.Request) *Ctx { return newCtx(w, r) }

// Request returns the request.
func (c *Ctx) Request() *http.Request { return c.r }

// Writer returns the response writer. A handler must not write to it. The
// router writes the Response after the transaction commits.
func (c *Ctx) Writer() http.ResponseWriter { return c.w }

// Context returns the request context. It carries the transaction and the
// session.
func (c *Ctx) Context() context.Context { return c.r.Context() }

// Deadline reports the time at which the request context ends.
//
// This method and the three below make *Ctx a context.Context, so a handler
// passes the Ctx itself to a store, to a client or to any function that takes
// a context. Each one reads the context of the current request, so a value
// that a middleware adds after the router built the Ctx is visible here.
//
//	rows, err := m.store.List(c, in.Page)
//
// A Ctx is valid for one request, exactly as an *http.Request is. Do not hold
// it after the handler returns.
func (c *Ctx) Deadline() (time.Time, bool) { return c.r.Context().Deadline() }

// Done returns the channel that closes when the request context ends.
func (c *Ctx) Done() <-chan struct{} { return c.r.Context().Done() }

// Err returns the reason that the request context ended, or nil.
func (c *Ctx) Err() error { return c.r.Context().Err() }

// Value returns the value that the request context carries for a key.
func (c *Ctx) Value(key any) any { return c.r.Context().Value(key) }

// setRequest replaces the request. A middleware that adds a context value
// calls it.
func (c *Ctx) setRequest(r *http.Request) { c.r = r }

// SetContext replaces the request context. A middleware outside this package
// calls it to add a value, such as the span that the telemetry middleware
// starts.
func (c *Ctx) SetContext(ctx context.Context) { c.r = c.r.WithContext(ctx) }

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
