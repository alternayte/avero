package router_test

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/alternayte/avero/router"
)

// syncBuffer is a buffer that a handler and a test can share.
type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func captureLogger() (*slog.Logger, func() string) {
	buf := &syncBuffer{}
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})), buf.String
}

// mark returns a middleware that records its name in order.
func mark(name string, seen *[]string, mu *sync.Mutex) router.Middleware {
	return router.Middleware{Name: name, Wrap: func(next router.Handler) router.Handler {
		return func(c *router.Ctx) (router.Response, error) {
			mu.Lock()
			*seen = append(*seen, name)
			mu.Unlock()
			return next(c)
		}
	}}
}

func TestMiddlewareRunsInRegistrationOrder(t *testing.T) {
	var seen []string
	var mu sync.Mutex
	r := router.New()
	r.Use(mark("first", &seen, &mu), mark("second", &seen, &mu))
	r.Get("/x", ok("done"))

	serve(t, r, http.MethodGet, "/x")
	if len(seen) != 2 || seen[0] != "first" || seen[1] != "second" {
		t.Fatalf("the chain ran %v, want [first second]", seen)
	}
}

func TestGroupMiddlewareAppliesInsideTheGroupOnly(t *testing.T) {
	var seen []string
	var mu sync.Mutex
	r := router.New()
	r.Use(mark("root", &seen, &mu))
	r.Group("/api", func(g *router.Router) {
		g.Use(mark("group", &seen, &mu))
		g.Get("/inside", ok("inside"))
	})
	r.Get("/outside", ok("outside"))

	serve(t, r, http.MethodGet, "/api/inside")
	if len(seen) != 2 || seen[0] != "root" || seen[1] != "group" {
		t.Fatalf("inside the group the chain ran %v, want [root group]", seen)
	}

	seen = nil
	serve(t, r, http.MethodGet, "/outside")
	if len(seen) != 1 || seen[0] != "root" {
		t.Fatalf("outside the group the chain ran %v, want [root]", seen)
	}
}

func TestASiblingGroupDoesNotSeeAnotherGroupMiddleware(t *testing.T) {
	var seen []string
	var mu sync.Mutex
	r := router.New()
	r.Group("/a", func(g *router.Router) {
		g.Use(mark("a", &seen, &mu))
		g.Get("/x", ok("a"))
	})
	r.Group("/b", func(g *router.Router) {
		g.Use(mark("b", &seen, &mu))
		g.Get("/x", ok("b"))
	})
	serve(t, r, http.MethodGet, "/b/x")
	if len(seen) != 1 || seen[0] != "b" {
		t.Fatalf("the chain ran %v, want [b]", seen)
	}
}

func TestMiddlewareRegisteredAfterARouteDoesNotApplyToIt(t *testing.T) {
	var seen []string
	var mu sync.Mutex
	r := router.New()
	r.Get("/early", ok("early"))
	r.Use(mark("late", &seen, &mu))
	r.Get("/late", ok("late"))

	serve(t, r, http.MethodGet, "/early")
	if len(seen) != 0 {
		t.Fatalf("the chain ran %v for a route registered before the middleware", seen)
	}
	serve(t, r, http.MethodGet, "/late")
	if len(seen) != 1 {
		t.Fatalf("the chain ran %v, want [late]", seen)
	}
}

func TestRequestIDReachesTheHandlerAndTheResponse(t *testing.T) {
	r := router.New()
	r.Use(router.RequestID())
	r.Get("/x", func(c *router.Ctx) (router.Response, error) {
		return router.Text(200, c.RequestID()), nil
	})
	rec := serve(t, r, http.MethodGet, "/x")
	if rec.Body.String() == "" {
		t.Fatal("the handler read no request ID")
	}
	if rec.Header().Get("X-Request-Id") != rec.Body.String() {
		t.Fatalf("the header is %q and the handler read %q",
			rec.Header().Get("X-Request-Id"), rec.Body.String())
	}
}

func TestRequestIDKeepsTheIDOfTheClient(t *testing.T) {
	r := router.New()
	r.Use(router.RequestID())
	r.Get("/x", func(c *router.Ctx) (router.Response, error) {
		return router.Text(200, c.RequestID()), nil
	})
	h, err := r.Handler()
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}
	req := newRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Request-Id", "from-the-client")
	rec := record(h, req)
	if rec.Body.String() != "from-the-client" {
		t.Fatalf("the handler read %q, want the id of the client", rec.Body.String())
	}
}

func TestRequestIDGivesADifferentIDToEachRequest(t *testing.T) {
	r := router.New()
	r.Use(router.RequestID())
	r.Get("/x", func(c *router.Ctx) (router.Response, error) {
		return router.Text(200, c.RequestID()), nil
	})
	first := serve(t, r, http.MethodGet, "/x").Body.String()
	second := serve(t, r, http.MethodGet, "/x").Body.String()
	if first == second {
		t.Fatalf("two requests share the id %q", first)
	}
}

func TestRecoverTurnsAPanicInto500(t *testing.T) {
	log, _ := captureLogger()
	r := router.New()
	r.Use(router.Recover(log))
	r.Get("/boom", func(*router.Ctx) (router.Response, error) { panic("the handler panicked") })

	rec := serve(t, r, http.MethodGet, "/boom")
	if rec.Code != 500 {
		t.Fatalf("gave %d, want 500", rec.Code)
	}
}

func TestRecoverDoesNotLeakThePanicValue(t *testing.T) {
	log, _ := captureLogger()
	r := router.New()
	r.Use(router.Recover(log))
	r.Get("/boom", func(*router.Ctx) (router.Response, error) { panic("secret-panic-value") })

	rec := serve(t, r, http.MethodGet, "/boom")
	if strings.Contains(rec.Body.String(), "secret-panic-value") {
		t.Fatalf("the response leaks the panic value: %q", rec.Body.String())
	}
}

func TestRecoverLogsThePanicAndTheStack(t *testing.T) {
	log, read := captureLogger()
	r := router.New()
	r.Use(router.Recover(log))
	r.Get("/boom", func(*router.Ctx) (router.Response, error) { panic("the handler panicked") })
	serve(t, r, http.MethodGet, "/boom")

	out := read()
	if !strings.Contains(out, "the handler panicked") {
		t.Fatalf("the log does not carry the panic value:\n%s", out)
	}
	if !strings.Contains(out, "stack") {
		t.Fatalf("the log does not carry the stack:\n%s", out)
	}
}

func TestAccessLogRecordsTheRequest(t *testing.T) {
	log, read := captureLogger()
	r := router.New()
	r.Use(router.RequestID(), router.AccessLog(log))
	r.Get("/things/{id}", ok("done"))
	serve(t, r, http.MethodGet, "/things/7")

	out := read()
	for _, want := range []string{"method=GET", "status=200", "path=/things/7", "request_id="} {
		if !strings.Contains(out, want) {
			t.Fatalf("the log line does not carry %s:\n%s", want, out)
		}
	}
}

func TestAdaptRunsANetHTTPMiddleware(t *testing.T) {
	r := router.New()
	r.Use(router.Adapt("stamp", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Stamp", "yes")
			next.ServeHTTP(w, req)
		})
	}))
	r.Get("/x", ok("done"))

	rec := serve(t, r, http.MethodGet, "/x")
	if rec.Header().Get("X-Stamp") != "yes" {
		t.Fatal("the adapted middleware did not run")
	}
	if rec.Body.String() != "done" {
		t.Fatalf("the handler gave %q", rec.Body.String())
	}
}

func TestAdaptLetsAMiddlewareAnswerTheRequest(t *testing.T) {
	reached := false
	r := router.New()
	r.Use(router.Adapt("reject", func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("no"))
		})
	}))
	r.Get("/x", func(*router.Ctx) (router.Response, error) {
		reached = true
		return router.Text(200, "done"), nil
	})

	rec := serve(t, r, http.MethodGet, "/x")
	if reached {
		t.Fatal("the handler ran although the middleware answered the request")
	}
	if rec.Code != http.StatusUnauthorized || rec.Body.String() != "no" {
		t.Fatalf("gave %d %q, want 401 \"no\"", rec.Code, rec.Body.String())
	}
}

func TestPartialReadsTheHypermediaHeaders(t *testing.T) {
	r := router.New()
	r.Get("/x", func(c *router.Ctx) (router.Response, error) {
		if c.Partial() {
			return router.Text(200, "fragment"), nil
		}
		return router.Text(200, "page"), nil
	})
	h, err := r.Handler()
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}
	for _, header := range []string{"HX-Request", "Datastar-Request"} {
		req := newRequest(http.MethodGet, "/x", nil)
		req.Header.Set(header, "true")
		if body := record(h, req).Body.String(); body != "fragment" {
			t.Fatalf("with %s the handler gave %q, want fragment", header, body)
		}
	}
	if body := record(h, newRequest(http.MethodGet, "/x", nil)).Body.String(); body != "page" {
		t.Fatalf("with no header the handler gave %q, want page", body)
	}
}

func TestToastsCollectInOrder(t *testing.T) {
	r := router.New()
	r.Get("/x", func(c *router.Ctx) (router.Response, error) {
		c.Success("saved")
		c.Warning("check the date")
		if got := c.Toasts(); len(got) != 2 ||
			got[0].Level != router.ToastSuccess || got[1].Message != "check the date" {
			t.Fatalf("the toasts are %+v", got)
		}
		return router.NoContent(), nil
	})
	serve(t, r, http.MethodGet, "/x")
}

func TestALongParentChainReachesAGroupInOrder(t *testing.T) {
	// A group joins the whole parent chain, in order, and then its own
	// middleware. The sibling group adds nothing to it.
	var seen []string
	var mu sync.Mutex
	r := router.New()
	r.Use(mark("root", &seen, &mu))
	r.Use(mark("root2", &seen, &mu), mark("root3", &seen, &mu), mark("root4", &seen, &mu))

	r.Group("/a", func(g *router.Router) {
		g.Use(mark("only-a", &seen, &mu))
		g.Get("/x", ok("a"))
	})
	r.Group("/b", func(g *router.Router) {
		g.Use(mark("only-b", &seen, &mu))
		g.Get("/x", ok("b"))
	})

	seen = nil
	serve(t, r, http.MethodGet, "/a/x")
	want := []string{"root", "root2", "root3", "root4", "only-a"}
	if len(seen) != len(want) {
		t.Fatalf("group a ran %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("group a ran %v, want %v", seen, want)
		}
	}
}
