package router_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alternayte/avero/router"
)

// serve builds the router and performs one request against it.
func serve(t *testing.T, r *router.Router, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	h, err := r.Handler()
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func ok(body string) router.Handler {
	return func(*router.Ctx) (router.Response, error) { return router.Text(200, body), nil }
}

func TestEachMethodRegistersItsPattern(t *testing.T) {
	r := router.New()
	r.Get("/things", ok("get"))
	r.Post("/things", ok("post"))
	r.Put("/things/{id}", ok("put"))
	r.Patch("/things/{id}", ok("patch"))
	r.Delete("/things/{id}", ok("delete"))

	for _, tc := range []struct{ method, target, want string }{
		{http.MethodGet, "/things", "get"},
		{http.MethodPost, "/things", "post"},
		{http.MethodPut, "/things/1", "put"},
		{http.MethodPatch, "/things/1", "patch"},
		{http.MethodDelete, "/things/1", "delete"},
	} {
		rec := serve(t, r, tc.method, tc.target)
		if rec.Code != 200 || rec.Body.String() != tc.want {
			t.Fatalf("%s %s gave %d %q, want 200 %q", tc.method, tc.target, rec.Code, rec.Body.String(), tc.want)
		}
	}
}

func TestAWrongMethodReturns405(t *testing.T) {
	r := router.New()
	r.Get("/things", ok("get"))
	if rec := serve(t, r, http.MethodPost, "/things"); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /things gave %d, want 405", rec.Code)
	}
}

func TestAnUnknownPatternReturns404(t *testing.T) {
	r := router.New()
	r.Get("/things", ok("get"))
	if rec := serve(t, r, http.MethodGet, "/absent"); rec.Code != http.StatusNotFound {
		t.Fatalf("GET /absent gave %d, want 404", rec.Code)
	}
}

func TestAPathValueReachesTheHandler(t *testing.T) {
	r := router.New()
	r.Get("/things/{id}", func(c *router.Ctx) (router.Response, error) {
		return router.Text(200, c.PathValue("id")), nil
	})
	if rec := serve(t, r, http.MethodGet, "/things/42"); rec.Body.String() != "42" {
		t.Fatalf("the handler read %q, want 42", rec.Body.String())
	}
}

func TestHandleTakesAnyMethod(t *testing.T) {
	r := router.New()
	r.Handle(http.MethodOptions, "/things", ok("options"))
	if rec := serve(t, r, http.MethodOptions, "/things"); rec.Body.String() != "options" {
		t.Fatalf("OPTIONS /things gave %q", rec.Body.String())
	}
}

func TestTwoHandlersOnOnePatternIsAFaultAtRegistration(t *testing.T) {
	r := router.New()
	r.Get("/things", ok("first"))
	r.Get("/things", ok("second"))

	h, err := r.Handler()
	if err == nil {
		t.Fatal("the router accepted two handlers on one pattern")
	}
	if h != nil {
		t.Fatal("Handler returned a handler beside the fault")
	}
	// The fault names the two registration sites, so a person finds both.
	msg := err.Error()
	if strings.Count(msg, "router_test.go:") < 2 {
		t.Fatalf("the fault does not name both registration sites:\n%s", msg)
	}
	if !strings.Contains(msg, "GET /things") {
		t.Fatalf("the fault does not name the pattern:\n%s", msg)
	}
	if !strings.Contains(msg, "→") {
		t.Fatalf("the fault states no repair:\n%s", msg)
	}
}

func TestTwoHandlersOnOnePatternInDifferentMethodsIsNoFault(t *testing.T) {
	r := router.New()
	r.Get("/things", ok("get"))
	r.Post("/things", ok("post"))
	if _, err := r.Handler(); err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}
}

func TestTheFaultAppearsBeforeAnyRequest(t *testing.T) {
	r := router.New()
	r.Get("/things", ok("first"))
	r.Get("/things", ok("second"))
	// A duplicate must never reach a request. net/http panics on a duplicate
	// registration, so the router must find it first.
	if _, err := r.Handler(); err == nil {
		t.Fatal("the router accepted two handlers on one pattern")
	}
}

func TestGroupPrefixesItsRoutes(t *testing.T) {
	r := router.New()
	r.Group("/api", func(g *router.Router) {
		g.Get("/things", ok("inside"))
	})
	r.Get("/things", ok("outside"))

	if rec := serve(t, r, http.MethodGet, "/api/things"); rec.Body.String() != "inside" {
		t.Fatalf("GET /api/things gave %q", rec.Body.String())
	}
	if rec := serve(t, r, http.MethodGet, "/things"); rec.Body.String() != "outside" {
		t.Fatalf("GET /things gave %q", rec.Body.String())
	}
}

func TestANestedGroupJoinsBothPrefixes(t *testing.T) {
	r := router.New()
	r.Group("/api", func(g *router.Router) {
		g.Group("/v1", func(g2 *router.Router) {
			g2.Get("/things", ok("nested"))
		})
	})
	if rec := serve(t, r, http.MethodGet, "/api/v1/things"); rec.Body.String() != "nested" {
		t.Fatalf("GET /api/v1/things gave %d %q", rec.Code, rec.Body.String())
	}
}

func TestMountServesAnHTTPHandlerUnderAPrefix(t *testing.T) {
	r := router.New()
	r.Mount("/admin", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte("mounted " + req.URL.Path))
	}))
	rec := serve(t, r, http.MethodGet, "/admin/users")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "mounted") {
		t.Fatalf("GET /admin/users gave %d %q", rec.Code, rec.Body.String())
	}
}

func TestMountStripsThePrefix(t *testing.T) {
	r := router.New()
	var got string
	r.Mount("/admin", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		got = req.URL.Path
		w.WriteHeader(204)
	}))
	serve(t, r, http.MethodGet, "/admin/users")
	if got != "/users" {
		t.Fatalf("the mounted handler saw %q, want /users", got)
	}
}

// KeepPrefix passes the whole path to a handler that removes its own base
// path, such as the handler of auth-all.
func TestMountWithKeepPrefixHoldsTheWholePath(t *testing.T) {
	r := router.New()
	var got string
	r.Mount("/api/auth", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		got = req.URL.Path
		w.WriteHeader(204)
	}), router.KeepPrefix())
	serve(t, r, http.MethodGet, "/api/auth/session")
	if got != "/api/auth/session" {
		t.Fatalf("the mounted handler saw %q, want /api/auth/session", got)
	}
}

// A group carries the mount, and the merge of a module keeps the option.
func TestMountWithKeepPrefixSurvivesAGroup(t *testing.T) {
	r := router.New()
	var got string
	r.Group("/api", func(g *router.Router) {
		g.Mount("/auth", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			got = req.URL.Path
			w.WriteHeader(204)
		}), router.KeepPrefix())
	})
	serve(t, r, http.MethodGet, "/api/auth/session")
	if got != "/api/auth/session" {
		t.Fatalf("the mounted handler saw %q, want /api/auth/session", got)
	}
}

// The middleware of the scope wraps a mounted handler, so a library that the
// application mounts carries the request identifier, the log and the session.
func TestTheMiddlewareOfTheScopeWrapsAMountedHandler(t *testing.T) {
	r := router.New()
	var order []string
	r.Use(router.Middleware{Name: "outer", Wrap: func(next router.Handler) router.Handler {
		return func(c *router.Ctx) (router.Response, error) {
			order = append(order, "outer")
			c.Writer().Header().Set("X-Outer", "1")
			return next(c)
		}
	}})
	r.Mount("/admin", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		order = append(order, "mounted")
		w.WriteHeader(201)
	}))
	rec := serve(t, r, http.MethodGet, "/admin/users")
	if rec.Code != 201 {
		t.Fatalf("GET /admin/users gave %d, want 201", rec.Code)
	}
	if rec.Header().Get("X-Outer") != "1" {
		t.Fatal("the middleware of the scope did not run")
	}
	if strings.Join(order, ",") != "outer,mounted" {
		t.Fatalf("the order is %v", order)
	}
}

// A middleware that acts on the response cannot hold its property around a
// handler that writes the answer itself, so a mount leaves it out. The
// transaction is the one that states it.
func TestAMountLeavesOutTheMiddlewareThatNeedsTheResponse(t *testing.T) {
	r := router.New()
	ran := false
	r.Use(router.Middleware{Name: "transaction", NeedsResponse: true, Wrap: func(next router.Handler) router.Handler {
		return func(c *router.Ctx) (router.Response, error) {
			ran = true
			return next(c)
		}
	}})
	r.Mount("/admin", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	r.Get("/things", ok("get"))

	if rec := serve(t, r, http.MethodGet, "/admin/users"); rec.Code != 204 {
		t.Fatalf("GET /admin/users gave %d, want 204", rec.Code)
	}
	if ran {
		t.Fatal("the middleware that needs the response wrapped the mount")
	}
	if rec := serve(t, r, http.MethodGet, "/things"); rec.Code != 200 {
		t.Fatalf("GET /things gave %d", rec.Code)
	}
	if !ran {
		t.Fatal("the middleware did not wrap a typed route")
	}
}

// The table of the routes prints the chain that the mount runs, and not the
// chain that it leaves out.
func TestTheReportNamesTheChainOfAMount(t *testing.T) {
	r := router.New()
	r.Use(router.Middleware{Name: "request-id", Wrap: func(next router.Handler) router.Handler { return next }})
	r.Use(router.Middleware{Name: "transaction", NeedsResponse: true, Wrap: func(next router.Handler) router.Handler { return next }})
	r.Mount("/admin", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	rep, err := r.Report()
	if err != nil {
		t.Fatalf("Report returned %v", err)
	}
	row := rep.Routes[0]
	if strings.Join(row.Middleware, ",") != "request-id" {
		t.Fatalf("the row names %v", row.Middleware)
	}
}

// A mounted handler of a stream asks the writer for http.Flusher, so the
// wrapper that records the status carries the method through. A wrapper that
// does not breaks a server sent event stream and a websocket.
func TestAMountedHandlerReachesTheFlush(t *testing.T) {
	r := router.New()
	r.Use(router.Middleware{Name: "outer", Wrap: func(next router.Handler) router.Handler { return next }})
	var flushed, controlled bool
	r.Mount("/stream", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, flushed = w.(http.Flusher)
		controlled = http.NewResponseController(w).Flush() == nil
		w.WriteHeader(200)
	}))
	serve(t, r, http.MethodGet, "/stream/updates")
	if !flushed {
		t.Fatal("the mounted handler cannot flush")
	}
	if !controlled {
		t.Fatal("http.NewResponseController does not reach the writer of the server")
	}
}

func TestAnEmptyPatternIsAFault(t *testing.T) {
	r := router.New()
	r.Get("", ok("x"))
	if _, err := r.Handler(); err == nil {
		t.Fatal("the router accepted an empty pattern")
	}
}

func TestAPatternWithNoLeadingSlashIsAFault(t *testing.T) {
	r := router.New()
	r.Get("things", ok("x"))
	if _, err := r.Handler(); err == nil {
		t.Fatal("the router accepted a pattern with no leading slash")
	}
}

func TestANilHandlerIsAFault(t *testing.T) {
	r := router.New()
	r.Get("/things", nil)
	if _, err := r.Handler(); err == nil {
		t.Fatal("the router accepted a nil handler")
	}
}

func TestAHandlerErrorReturns500(t *testing.T) {
	r := router.New()
	r.Get("/boom", func(*router.Ctx) (router.Response, error) {
		return nil, errBoom
	})
	if rec := serve(t, r, http.MethodGet, "/boom"); rec.Code != 500 {
		t.Fatalf("GET /boom gave %d, want 500", rec.Code)
	}
}

func TestAHandlerErrorDoesNotLeakItsMessage(t *testing.T) {
	r := router.New()
	r.Get("/boom", func(*router.Ctx) (router.Response, error) {
		return nil, errBoom
	})
	rec := serve(t, r, http.MethodGet, "/boom")
	if strings.Contains(rec.Body.String(), errBoom.Error()) {
		t.Fatalf("the response leaks the error message: %q", rec.Body.String())
	}
}

// newRequest builds a request for a test.
func newRequest(method, target string, body io.Reader) *http.Request {
	return httptest.NewRequest(method, target, body)
}

// record runs one request against a built handler.
func record(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// The zero middleware adds no frame, so a constructor that states none, such
// as db.Transaction with a nil engine, needs no guard at the call site.
func TestUseIgnoresTheZeroMiddleware(t *testing.T) {
	r := router.New()
	r.Use(router.Middleware{})
	r.Get("/x", ok("x"))
	if rec := serve(t, r, http.MethodGet, "/x"); rec.Code != 200 {
		t.Fatalf("GET /x gave %d", rec.Code)
	}
}

// net/http fills Request.Pattern during the dispatch of the mux. The router
// registers each route with its own chain below that dispatch, so a middleware
// reads the pattern of the route that matched. A metric label and a span name
// therefore name the route and never the raw path.
func TestTheMiddlewareReadsThePatternOfTheRoute(t *testing.T) {
	r := router.New()
	var typed, mounted string
	r.Use(router.Middleware{Name: "probe", Wrap: func(next router.Handler) router.Handler {
		return func(c *router.Ctx) (router.Response, error) {
			typed = c.Request().Pattern
			return next(c)
		}
	}})
	r.Get("/posts/{id}", ok("x"))
	r.Mount("/admin", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mounted = req.Pattern
		w.WriteHeader(204)
	}))

	serve(t, r, http.MethodGet, "/posts/7")
	if typed != "GET /posts/{id}" {
		t.Fatalf("the middleware read the pattern %q", typed)
	}
	serve(t, r, http.MethodGet, "/admin/users")
	if mounted != "/admin/" {
		t.Fatalf("the mounted handler read the pattern %q", mounted)
	}
}
