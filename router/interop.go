package router

import (
	"fmt"
	"net/http"
)

// This file carries the way out of Avero. Adapt carries the way in: it turns a
// net/http middleware into an Avero one. The three functions here turn an
// Avero handler, an Avero middleware and an Avero router into the types of
// net/http, so an application that holds a mux of its own adopts one part of
// Avero and keeps the rest of its stack.

// HTTP returns the handler as an http.Handler, so a plain mux, a chi router or
// any other library serves it.
//
//	mux.Handle("GET /posts", avero.In(m.List).HTTP())
//
// The handler writes the Response itself, because no router surrounds it. An
// error becomes the problem document that the error carries, which is the
// default of the router. A route that needs another writer of errors belongs
// on an Avero router, which states one for every route.
//
// A validation fault answers the default 422 problem document. The option
// WithValidationResponse states a router, so it cannot reach a handler that
// stands alone.
func (h Handler) HTTP() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := newCtx(w, r)
		res, err := h(c)
		if err != nil {
			res = defaultErrorResponse(c, err)
		}
		if res == nil {
			res = NoContent()
		}
		// The status is already sent when Write fails. Nothing more can reach
		// the client.
		_ = res.Write(c)
	})
}

// HTTP returns the middleware in the shape of net/http, so a stack of another
// library runs it.
//
//	mux.Handle("/", avero.RequestID().HTTP()(next))
//
// The middleware sees a handler that serves the rest of the chain and returns
// a Response that carries the status that the chain wrote. A middleware that
// answers the request itself, such as the CSRF check, writes its Response and
// the rest of the chain never runs.
//
// The zero middleware returns the handler that it received, so a field of
// Stack that an application leaves empty adds no frame.
//
// net/http fills Request.Pattern when the mux dispatches the request. A
// middleware that wraps a whole mux therefore reads no pattern: it runs before
// the dispatch. A middleware that needs the route, such as the telemetry of a
// metric label, belongs on an Avero router, which wraps each route below its
// own dispatch.
func (m Middleware) HTTP() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if m.Wrap == nil {
			return next
		}
		wrapped := m.Wrap(func(c *Ctx) (Response, error) {
			watch := &statusWriter{ResponseWriter: c.Writer()}
			next.ServeHTTP(watch, c.Request())
			return alreadyWritten{status: watch.status()}, nil
		})
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := newCtx(w, r)
			res, err := wrapped(c)
			if err != nil {
				res = defaultErrorResponse(c, err)
			}
			if res == nil {
				res = NoContent()
			}
			_ = res.Write(c)
		})
	}
}

// MustHandler builds the http.Handler and panics on a registration fault.
//
// Handler returns the faults, and `main` reports them, which is the form that
// an application uses. MustHandler is for the caller that has no place for an
// error: the argument of another library.
//
//	mux.Handle("/api/", http.StripPrefix("/api", api.MustHandler()))
//
// The panic carries the same text that Handler returns: the file, the line and
// the repair of each fault. A registration fault is a fault of the program, so
// it appears before the process serves. See DX-8.
func (r *Router) MustHandler() http.Handler {
	h, err := r.Handler()
	if err != nil {
		panic(fmt.Sprintf("avero: the router holds a registration fault\n%v", err))
	}
	return h
}
