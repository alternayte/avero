package router_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alternayte/avero/router"
)

// key is the type of a context key of this test.
type key struct{}

// take proves at compile time that a Ctx passes where a context passes.
func take(ctx context.Context) any { return ctx.Value(key{}) }

func TestCtxIsAContext(t *testing.T) {
	var _ context.Context = (*router.Ctx)(nil)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req = req.WithContext(context.WithValue(req.Context(), key{}, "carried"))
	c := router.NewCtx(httptest.NewRecorder(), req)

	if got := take(c); got != "carried" {
		t.Fatalf("the value is %v, want carried", got)
	}
	if c.Err() != nil {
		t.Fatalf("Err is %v, want nil", c.Err())
	}
	if _, ok := c.Deadline(); ok {
		t.Fatal("Deadline states a time although the request carries none")
	}
	select {
	case <-c.Done():
		t.Fatal("Done is closed although the request runs")
	default:
	}
}

func TestCtxReadsTheContextOfTheCurrentRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	c := router.NewCtx(httptest.NewRecorder(), req)

	// A middleware replaces the context after the router built the Ctx. The
	// four methods must read the new value and not a copy of the old one.
	c.SetContext(context.WithValue(c.Context(), key{}, "later"))
	if got := take(c); got != "later" {
		t.Fatalf("the value is %v, want later", got)
	}

	ctx, cancel := context.WithTimeout(c.Context(), time.Hour)
	defer cancel()
	c.SetContext(ctx)
	if _, ok := c.Deadline(); !ok {
		t.Fatal("Deadline states no time although the context carries one")
	}
	cancel()
	<-c.Done()
	if c.Err() == nil {
		t.Fatal("Err is nil although the context ended")
	}
}
