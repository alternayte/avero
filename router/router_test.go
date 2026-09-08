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
