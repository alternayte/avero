package router

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
)

// Middleware wraps a handler. It carries a name, because `avero routes` prints
// the chain of each route. See the SDD, S4.
type Middleware struct {
	// Name identifies the middleware in the routes output.
	Name string
	// Wrap returns a handler that calls next.
	Wrap func(next Handler) Handler
	// NeedsResponse states a middleware that acts on the response before one
	// byte reaches the client. Transaction sets it, because it commits from
	// the status and a commit must stand before the first byte.
	//
	// A mounted handler writes to the ResponseWriter itself, so such a
	// middleware cannot hold its property there. Mount leaves it out and
	// runs every other middleware of the scope. See Router.Mount.
	NeedsResponse bool
}

// mountable returns the middleware that a mounted handler runs.
func mountable(mws []Middleware) []Middleware {
	out := make([]Middleware, 0, len(mws))
	for _, mw := range mws {
		if mw.NeedsResponse {
			continue
		}
		out = append(out, mw)
	}
	return out
}

// chain applies the middleware in registration order. The first registered
// middleware is the outermost one.
func chain(h Handler, mws []Middleware) Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i].Wrap(h)
	}
	return h
}

// Adapt turns a net/http middleware into an Avero middleware, so that a
// library such as auth-all composes with a typed handler.
//
// The adapted middleware sees a handler that runs the rest of the chain and
// writes nothing. When it calls that handler, Adapt returns the Response that
// the chain produced. When it answers the request itself, for example a 401
// from RequireAuth, Adapt returns a response that writes nothing more, because
// the middleware already wrote the whole answer.
//
// Adapt watches the writer that it gives to the middleware, so the Response
// that it returns carries the status that the middleware wrote. The
// transaction middleware reads that status, so a middleware that writes a row
// and then answers 500 rolls back. A middleware that writes no status answers
// 200, as net/http does.
func Adapt(name string, mw func(http.Handler) http.Handler) Middleware {
	return Middleware{
		Name: name,
		Wrap: func(next Handler) Handler {
			return func(c *Ctx) (Response, error) {
				var (
					res    Response
					err    error
					called bool
				)
				watch := &statusWriter{ResponseWriter: c.Writer()}
				inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
					called = true
					c.setRequest(r)
					res, err = next(c)
				})
				mw(inner).ServeHTTP(watch, c.Request())
				if !called {
					// The middleware answered the request itself.
					return alreadyWritten{status: watch.status()}, nil
				}
				return res, err
			}
		},
	}
}

// statusWriter records the status that a net/http middleware wrote.
type statusWriter struct {
	http.ResponseWriter
	code int
}

// WriteHeader records the status and sends it.
func (w *statusWriter) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
	}
	w.ResponseWriter.WriteHeader(code)
}

// Write records the implicit status of a body that carries no status.
func (w *statusWriter) Write(p []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	return w.ResponseWriter.Write(p)
}

// Unwrap returns the writer below, so http.NewResponseController reaches the
// flush, the deadline and the hijack of the server.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Flush sends the buffered bytes. A handler of a stream, such as a server sent
// event stream, asks the writer for http.Flusher, so the wrapper carries the
// method through.
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		if w.code == 0 {
			w.code = http.StatusOK
		}
		f.Flush()
	}
}

// Hijack gives the connection to a handler that speaks another protocol, such
// as a websocket.
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("router: the response writer does not hijack")
	}
	return h.Hijack()
}

// status returns the status that the middleware wrote. A middleware that wrote
// none answers 200, as net/http does.
func (w *statusWriter) status() int {
	if w.code == 0 {
		return http.StatusOK
	}
	return w.code
}

// alreadyWritten reports that a net/http middleware wrote the whole response.
// The router writes nothing more. It carries the status that the middleware
// wrote, so the transaction middleware commits a rejection such as a 401 and
// rolls back a fault such as a 500.
type alreadyWritten struct{ status int }

func (a alreadyWritten) Status() int    { return a.status }
func (alreadyWritten) Write(*Ctx) error { return nil }
