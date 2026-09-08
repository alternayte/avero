// Package router builds an http.Handler from typed handlers.
//
// It uses net/http and the pattern syntax of Go 1.22. It adds no third-party
// router. See the SDD, S4.
//
// A handler returns a Response and never writes to the ResponseWriter, so the
// transaction middleware commits before anything reaches the client.
package router

import (
	"fmt"
	"net/http"
	"path"
	"reflect"
	"runtime"
	"strings"
)

// Handler is one typed handler. It returns the response and an error.
type Handler func(c *Ctx) (Response, error)

// Router registers routes and builds the http.Handler that serves them.
//
// A Router value is one scope. Group returns a child scope that carries a
// prefix and its own middleware. A child never changes its parent.
type Router struct {
	prefix string
	mws    []Middleware
	reg    *registry
}

// registry holds the state that every scope of one router shares.
type registry struct {
	routes    []Route
	faults    []*Fault
	seen      map[string]Route
	onErr     func(c *Ctx, err error) Response
	onInvalid func(c *Ctx, f *Fields) Response
}

// New builds a router.
func New(opts ...Option) *Router {
	r := &Router{reg: &registry{
		seen:  make(map[string]Route),
		onErr: defaultErrorResponse,
	}}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Option configures a router.
type Option func(*Router)

// WithErrorResponse sets the response that a handler error produces. The
// default answers 500 and never prints the message of the error.
func WithErrorResponse(fn func(c *Ctx, err error) Response) Option {
	return func(r *Router) { r.reg.onErr = fn }
}

// WithValidationResponse sets the response that a validation fault produces.
// The default answers 422 with a map of field name to message. The SSR shape
// replaces it, so the middleware renders the form again with the errors and
// the old input. See the SDD, S5.
func WithValidationResponse(fn func(c *Ctx, f *Fields) Response) Option {
	return func(r *Router) { r.reg.onInvalid = fn }
}

// defaultErrorResponse answers 500. It never writes the message of the error,
// because a handler error can carry an internal detail.
func defaultErrorResponse(_ *Ctx, _ error) Response {
	return Status(http.StatusInternalServerError)
}

// Use adds middleware to this scope. It applies to every route that this scope
// registers after the call, and to every child group.
func (r *Router) Use(mws ...Middleware) { r.mws = append(r.mws, mws...) }

// Get registers a GET route.
func (r *Router) Get(pattern string, h Handler) { r.register(http.MethodGet, pattern, h) }

// Post registers a POST route.
func (r *Router) Post(pattern string, h Handler) { r.register(http.MethodPost, pattern, h) }

// Put registers a PUT route.
func (r *Router) Put(pattern string, h Handler) { r.register(http.MethodPut, pattern, h) }

// Patch registers a PATCH route.
func (r *Router) Patch(pattern string, h Handler) { r.register(http.MethodPatch, pattern, h) }

// Delete registers a DELETE route.
func (r *Router) Delete(pattern string, h Handler) { r.register(http.MethodDelete, pattern, h) }

// Handle registers a route for any method.
func (r *Router) Handle(method, pattern string, h Handler) { r.register(method, pattern, h) }

// Group returns a child scope with a prefix. The middleware that the child
// adds applies inside the group only.
//
//	r.Group("/api", func(g *router.Router) {
//	    g.Use(router.RequireJSON())
//	    g.Get("/things", listThings)
//	})
func (r *Router) Group(prefix string, fn func(g *Router)) {
	// The child copies the chain of its parent. register copies it again for
	// each route, so a later Use on a sibling group can never reach a route
	// that is already registered.
	child := &Router{
		prefix: joinPattern(r.prefix, prefix),
		mws:    append([]Middleware(nil), r.mws...),
		reg:    r.reg,
	}
	fn(child)
}

// Mount serves an http.Handler under a prefix. The handler sees the path with
// the prefix removed.
//
// A mounted handler writes to the ResponseWriter itself, so no transaction
// surrounds it. `avero routes` marks it.
func (r *Router) Mount(prefix string, h http.Handler) {
	full := joinPattern(r.prefix, prefix)
	file, line := caller(1)
	if h == nil {
		r.fault(file, line, fmt.Sprintf("the mount on %s has no handler", full),
			"Pass an http.Handler to Mount")
		return
	}
	route := Route{
		Method:     "*",
		Pattern:    strings.TrimSuffix(full, "/") + "/",
		Handler:    handlerName(h),
		Middleware: names(r.mws),
		File:       file,
		Line:       line,
		Mounted:    true,
		stripped:   strings.TrimSuffix(full, "/"),
		mount:      h,
	}
	r.add(route)
}

// register records one route. It records a fault instead of the route when the
// registration is wrong, so that every fault reaches one report.
func (r *Router) register(method, pattern string, h Handler) {
	full := joinPattern(r.prefix, pattern)
	file, line := caller(2)
	switch {
	case pattern == "":
		r.fault(file, line, fmt.Sprintf("the %s route has an empty pattern", method),
			"Give the route a pattern that starts with a slash, such as `/things`")
		return
	case !strings.HasPrefix(pattern, "/"):
		r.fault(file, line, fmt.Sprintf("the %s pattern %q does not start with a slash", method, pattern),
			fmt.Sprintf("Write the pattern as `/%s`", pattern))
		return
	case h == nil:
		r.fault(file, line, fmt.Sprintf("the %s route on %s has no handler", method, full),
			"Pass a handler to the route, or delete the registration")
		return
	}
	r.add(Route{
		Method:     method,
		Pattern:    full,
		Handler:    handlerName(h),
		Middleware: names(r.mws),
		File:       file,
		Line:       line,
		handler:    h,
		mws:        append([]Middleware(nil), r.mws...),
	})
}

// add records a route and reports a duplicate. net/http panics on a duplicate
// registration, so the router finds it first and reports both sites.
func (r *Router) add(route Route) {
	key := route.Method + " " + route.Pattern
	if first, ok := r.reg.seen[key]; ok {
		r.fault(route.File, route.Line,
			fmt.Sprintf("%s is already registered at %s:%d", key, first.File, first.Line),
			"Delete one of the two registrations, or give one of them a different pattern")
		return
	}
	r.reg.seen[key] = route
	r.reg.routes = append(r.reg.routes, route)
}

// fault records a registration fault.
func (r *Router) fault(file string, line int, message, repair string) {
	r.reg.faults = append(r.reg.faults, &Fault{File: file, Line: line, Message: message, Repair: repair})
}

// Handler builds the http.Handler. It returns every registration fault and no
// handler, so a fault stops the process before it serves. See DX-8.
func (r *Router) Handler() (http.Handler, error) {
	if len(r.reg.faults) > 0 {
		return nil, &Faults{Faults: r.reg.faults}
	}
	mux := http.NewServeMux()
	for _, route := range r.reg.routes {
		mux.Handle(route.muxPattern(), r.serve(route))
	}
	return mux, nil
}

// serve turns one route into an http.Handler.
func (r *Router) serve(route Route) http.Handler {
	if route.Mounted {
		return http.StripPrefix(route.stripped, route.mount)
	}
	h := chain(route.handler, route.mws)
	onErr := r.reg.onErr
	onInvalid := r.reg.onInvalid
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		c := newCtx(w, req)
		c.onInvalid = onInvalid
		res, err := h(c)
		if err != nil {
			res = onErr(c, err)
		}
		if res == nil {
			res = NoContent()
		}
		if writeErr := res.Write(c); writeErr != nil {
			// The status is already sent. Nothing more can reach the client.
			return
		}
	})
}

// muxPattern returns the pattern in the net/http 1.22 form.
func (route Route) muxPattern() string {
	if route.Mounted {
		return route.Pattern
	}
	return route.Method + " " + route.Pattern
}

// joinPattern joins a prefix and a pattern.
func joinPattern(prefix, pattern string) string {
	switch {
	case prefix == "":
		return pattern
	case pattern == "" || pattern == "/":
		return prefix
	}
	return path.Join(prefix, pattern)
}

// names returns the name of each middleware.
func names(mws []Middleware) []string {
	if len(mws) == 0 {
		return nil
	}
	out := make([]string, 0, len(mws))
	for _, m := range mws {
		out = append(out, m.Name)
	}
	return out
}

// handlerName returns the name of a function. It reads the name one time at
// registration. No request path calls it. See design rule 2.
func handlerName(v any) string {
	if v == nil {
		return ""
	}
	fn := runtime.FuncForPC(reflect.ValueOf(v).Pointer())
	if fn == nil {
		return fmt.Sprintf("%T", v)
	}
	name := fn.Name()
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return strings.TrimSuffix(name, "-fm")
}

// caller returns the file and the line of the code that registered a route. It
// names the file that the person wrote, not a file inside Avero. See DX-6.
//
// skip counts the frames between the code that the person wrote and the
// function that calls caller. register sits two frames below it, because Get
// and its siblings call register. Mount sits one frame below it.
func caller(skip int) (string, int) {
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "", 0
	}
	return file, line
}
