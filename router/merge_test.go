package router_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/alternayte/avero/router"
)

func TestMergeCopiesTheRoutesAndKeepsThePrefix(t *testing.T) {
	src := router.New()
	src.Get("/things", ok("things"))
	src.Mount("/static", http.NotFoundHandler())

	dst := router.New()
	dst.Use(router.Middleware{Name: "requestid", Wrap: func(next router.Handler) router.Handler { return next }})
	dst.Group("/api", func(g *router.Router) { g.Merge(src) })

	rep, err := dst.Report()
	if err != nil {
		t.Fatalf("Report returned %v, want nil", err)
	}
	row, found := rep.Route("GET", "/api/things")
	if !found {
		t.Fatalf("the router holds no GET /api/things:\n%s", rep)
	}
	if strings.Join(row.Middleware, ",") != "requestid" {
		t.Fatalf("Middleware holds %v, want requestid", row.Middleware)
	}
	if _, found := rep.Route("*", "/api/static/"); !found {
		t.Fatalf("the router holds no mounted /api/static/:\n%s", rep)
	}
	if _, err := dst.Handler(); err != nil {
		t.Fatalf("Handler returned %v, want nil", err)
	}
}

func TestMergeReportsADuplicatePattern(t *testing.T) {
	src := router.New()
	src.Get("/things", ok("first"))

	dst := router.New()
	dst.Get("/things", ok("second"))
	dst.Merge(src)

	if _, err := dst.Report(); err == nil {
		t.Fatal("Report accepted two handlers on one pattern")
	}
}

func TestMergeCopiesTheFaults(t *testing.T) {
	src := router.New()
	src.Get("things", ok("no slash"))

	dst := router.New()
	dst.Merge(src)

	if len(dst.Faults()) != 1 {
		t.Fatalf("Faults holds %d rows, want 1", len(dst.Faults()))
	}
}

func TestRoutesReturnsTheRegistrationOrder(t *testing.T) {
	r := router.New()
	r.Get("/b", ok("b"))
	r.Get("/a", ok("a"))
	rows := r.Routes()
	if len(rows) != 2 || rows[0].Pattern != "/b" {
		t.Fatalf("Routes holds %v, want the registration order", rows)
	}
}

func TestMergeAcceptsANilRouter(t *testing.T) {
	r := router.New()
	r.Merge(nil)
	if len(r.Routes()) != 0 {
		t.Fatalf("Routes holds %d rows, want 0", len(r.Routes()))
	}
}
