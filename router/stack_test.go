package router_test

import (
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/alternayte/avero/config"
	"github.com/alternayte/avero/router"
)

// testSecret is a key of the correct length.
var testSecret = config.Secret(strings.Repeat("0", 64))

func TestStackHoldsTheOrderOfTheSpecification(t *testing.T) {
	trace := router.Middleware{Name: "trace", Wrap: func(next router.Handler) router.Handler { return next }}
	session := router.Adapt("session", func(h http.Handler) http.Handler { return h })
	auth := router.Adapt("auth", func(h http.Handler) http.Handler { return h })

	got := names(router.Stack{
		Secret:  testSecret,
		Logger:  slog.Default(),
		Trace:   trace,
		Session: []router.Middleware{session},
		Auth:    []router.Middleware{auth},
	}.Middleware())

	want := []string{
		"request_id", "recover", "access_log", "trace",
		"flash", "session", "csrf", "auth",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the stack is %v, want %v", got, want)
	}
}

func TestStackOmitsThePartsThatItHasNoValueFor(t *testing.T) {
	got := names(router.Stack{Secret: testSecret}.Middleware())
	want := []string{"request_id", "recover", "flash", "csrf"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the stack is %v, want %v", got, want)
	}
}

func TestStackServesARequest(t *testing.T) {
	r := router.New()
	r.Use(router.Stack{Secret: testSecret}.Middleware()...)
	r.Get("/x", ok("done"))

	rec := serve(t, r, http.MethodGet, "/x")
	if rec.Code != http.StatusOK || rec.Body.String() != "done" {
		t.Fatalf("the stack gave %d %q", rec.Code, rec.Body.String())
	}
}

// names returns the name of each middleware of a chain.
func names(mws []router.Middleware) []string {
	out := make([]string, 0, len(mws))
	for _, mw := range mws {
		out = append(out, mw.Name)
	}
	return out
}

func TestStackAPILeavesTheCookiesOut(t *testing.T) {
	got := names(router.Stack{Secret: testSecret, Logger: slog.Default()}.API())
	want := []string{"request_id", "recover", "access_log"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the API stack is %v, want %v", got, want)
	}
}
