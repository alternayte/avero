package router_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alternayte/avero/router"
)

// An application that holds a mux of its own serves one Avero handler, so it
// adopts the typed answer without the router.
func TestAHandlerServesOnAPlainMux(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /posts", router.Handler(func(_ *router.Ctx) (router.Response, error) {
		return router.JSON(200, map[string]string{"title": "one"}), nil
	}).HTTP())

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/posts", nil))
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"title":"one"}` {
		t.Fatalf("GET /posts gave %d %q", rec.Code, rec.Body.String())
	}
}

// A handler that returns a Problem answers the problem document, because the
// default writer of errors stands with no router.
func TestAHandlerOnAPlainMuxAnswersTheProblem(t *testing.T) {
	h := router.Handler(func(_ *router.Ctx) (router.Response, error) {
		return nil, router.NotFound("post", "7")
	}).HTTP()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/posts/7", nil))
	if rec.Code != 404 || rec.Header().Get("Content-Type") != router.ProblemContentType {
		t.Fatalf("the answer is %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
}

// The middleware of Avero runs in the stack of another library.
func TestMiddlewareRunsInAStackOfNetHTTP(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(201)
	})
	h := router.RequestID().HTTP()(inner)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != 201 {
		t.Fatalf("the answer is %d", rec.Code)
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Fatal("the middleware wrote no request identifier")
	}
}

// A middleware that answers the request itself stops the chain, and the
// handler below it never runs.
func TestAMiddlewareOfNetHTTPShapeCanAnswerItself(t *testing.T) {
	reached := false
	inner := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true })
	stop := router.Middleware{Name: "stop", Wrap: func(_ router.Handler) router.Handler {
		return func(_ *router.Ctx) (router.Response, error) { return router.Status(403), nil }
	}}
	rec := httptest.NewRecorder()
	stop.HTTP()(inner).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != 403 {
		t.Fatalf("the answer is %d", rec.Code)
	}
	if reached {
		t.Fatal("the chain ran below a middleware that answered")
	}
}

// The zero middleware adds no frame, so an empty field of Stack costs nothing.
func TestTheZeroMiddlewareReturnsTheHandler(t *testing.T) {
	inner := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	if got := (router.Middleware{}).HTTP()(inner); got == nil {
		t.Fatal("the zero middleware returned no handler")
	}
}

// Another library mounts an Avero router, and a registration fault panics with
// the text that Handler returns.
func TestMustHandlerServesAndPanicsOnAFault(t *testing.T) {
	api := router.New()
	api.Get("/things", ok("things"))
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", api.MustHandler()))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/things", nil))
	if rec.Code != 200 || rec.Body.String() != "things" {
		t.Fatalf("GET /api/things gave %d %q", rec.Code, rec.Body.String())
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("MustHandler accepted a registration fault")
		}
	}()
	bad := router.New()
	bad.Get("things", ok("x"))
	_ = bad.MustHandler()
}
