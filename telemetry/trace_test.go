package telemetry_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alternayte/avero/config"
	"github.com/alternayte/avero/router"
	"github.com/alternayte/avero/telemetry"
	"github.com/alternayte/drel"
	"go.opentelemetry.io/otel/trace"
)

func TestARequestProducesASpanNamedByTheRoutePattern(t *testing.T) {
	p, _, exp := newRecorder(t)
	r := router.New()
	r.Use(p.HTTPMiddleware())
	r.Get("/things/{id}", func(*router.Ctx) (router.Response, error) {
		return router.Text(200, "done"), nil
	})
	serve(t, r, http.MethodGet, "/things/7")

	span := spanNamed(t, exp, "GET /things/{id}")
	if span.SpanKind != trace.SpanKindServer {
		t.Fatalf("the span kind is %v, want server", span.SpanKind)
	}
	attrs := map[string]string{}
	for _, a := range span.Attributes {
		attrs[string(a.Key)] = a.Value.Emit()
	}
	if attrs["http.request.method"] != "GET" {
		t.Fatalf("the span does not carry the method: %v", attrs)
	}
	if attrs["http.route"] != "/things/{id}" {
		t.Fatalf("the span carries route %q, want the pattern", attrs["http.route"])
	}
	if attrs["http.response.status_code"] != "200" {
		t.Fatalf("the span carries status %q, want 200", attrs["http.response.status_code"])
	}
}

func TestASpanRecordsAServerFault(t *testing.T) {
	p, _, exp := newRecorder(t)
	r := router.New()
	r.Use(p.HTTPMiddleware())
	r.Get("/boom", func(*router.Ctx) (router.Response, error) {
		return router.Status(503), nil
	})
	serve(t, r, http.MethodGet, "/boom")

	span := spanNamed(t, exp, "GET /boom")
	if span.Status.Code.String() != "Error" {
		t.Fatalf("the span status is %v, want Error", span.Status.Code)
	}
}

func TestA4xxIsNotASpanFault(t *testing.T) {
	// A 404 is the answer of the server, not a fault of the server.
	p, _, exp := newRecorder(t)
	r := router.New()
	r.Use(p.HTTPMiddleware())
	r.Get("/gone", func(*router.Ctx) (router.Response, error) {
		return router.Status(404), nil
	})
	serve(t, r, http.MethodGet, "/gone")

	span := spanNamed(t, exp, "GET /gone")
	if span.Status.Code.String() == "Error" {
		t.Fatalf("a 404 marked the span as a fault")
	}
}

func TestAQuerySpanIsAChildOfItsRequestSpan(t *testing.T) {
	p, _, exp := newRecorder(t)
	e := newEngine(t, p)
	// newEngine writes the schema in its own transaction, which produces a
	// span with no parent. Drop it, so that the test reads the request only.
	exp.Reset()

	r := router.New()
	r.Use(p.HTTPMiddleware(), router.Transaction(e))
	r.Post("/things", func(c *router.Ctx) (router.Response, error) {
		if _, err := c.MustTx().Exec(c.Context(), `INSERT INTO things (name) VALUES (?)`, "one"); err != nil {
			return nil, err
		}
		return router.Text(201, "made"), nil
	})
	if rec := serve(t, r, http.MethodPost, "/things"); rec.Code != 201 {
		t.Fatalf("gave %d, want 201", rec.Code)
	}

	request := spanNamed(t, exp, "POST /things")
	query := spanNamed(t, exp, "drel.exec")

	if query.Parent.SpanID() != request.SpanContext.SpanID() {
		t.Fatalf("the query span has parent %s, want the request span %s",
			query.Parent.SpanID(), request.SpanContext.SpanID())
	}
	if query.SpanContext.TraceID() != request.SpanContext.TraceID() {
		t.Fatal("the query span sits in another trace")
	}
}

func TestASecretNeverAppearsInASpanAttribute(t *testing.T) {
	p, _, exp := newRecorder(t)
	r := router.New()
	r.Use(p.HTTPMiddleware())
	r.Get("/things", func(*router.Ctx) (router.Response, error) {
		return router.Text(200, "done"), nil
	})
	h, err := r.Handler()
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}

	// The secret arrives in the query string, in a header and in a cookie.
	req := httptest.NewRequest(http.MethodGet, "/things?token="+theSecret, nil)
	req.Header.Set("Authorization", "Bearer "+theSecret)
	req.Header.Set("Cookie", "session="+theSecret)
	h.ServeHTTP(httptest.NewRecorder(), req)

	for _, span := range exp.GetSpans() {
		if strings.Contains(span.Name, theSecret) {
			t.Fatalf("the span name carries the secret: %q", span.Name)
		}
		for _, a := range span.Attributes {
			if strings.Contains(a.Value.Emit(), theSecret) {
				t.Fatalf("the attribute %s carries the secret", a.Key)
			}
		}
	}
}

func TestASecretNeverAppearsInAQuerySpan(t *testing.T) {
	p, _, exp := newRecorder(t)
	e := newEngine(t, p)
	exp.Reset()

	err := e.WithTx(context.Background(), func(ctx context.Context) error {
		_, execErr := drel.MustFromContext(ctx).
			Exec(ctx, `INSERT INTO things (name) VALUES (?)`, theSecret)
		return execErr
	})
	if err != nil {
		t.Fatalf("the write failed: %v", err)
	}

	if len(exp.GetSpans()) == 0 {
		t.Fatal("the engine produced no span")
	}
	for _, span := range exp.GetSpans() {
		if strings.Contains(span.Name, theSecret) {
			t.Fatalf("the span name carries the secret: %q", span.Name)
		}
		for _, a := range span.Attributes {
			if strings.Contains(a.Value.Emit(), theSecret) {
				t.Fatalf("the attribute %s carries the secret", a.Key)
			}
		}
	}
}

func TestAttrRedactsASecret(t *testing.T) {
	a := telemetry.Attr("api_key", config.Secret(theSecret))
	if a.Value.Emit() != config.Redacted {
		t.Fatalf("the attribute reads %q, want %q", a.Value.Emit(), config.Redacted)
	}
}

func TestAttrKeepsAnOrdinaryValue(t *testing.T) {
	for _, tc := range []struct {
		value any
		want  string
	}{
		{"orders", "orders"},
		{7, "7"},
		{true, "true"},
	} {
		if got := telemetry.Attr("k", tc.value).Value.Emit(); got != tc.want {
			t.Fatalf("Attr(%v) gave %q, want %q", tc.value, got, tc.want)
		}
	}
}

func TestASecretInASpanNeverReachesTheExporter(t *testing.T) {
	p, _, exp := newRecorder(t)
	ctx, span := p.StartClient(context.Background(), "payments.charge",
		telemetry.Attr("api_key", config.Secret(theSecret)))
	_ = ctx
	span.End()

	for _, s := range exp.GetSpans() {
		for _, a := range s.Attributes {
			if strings.Contains(a.Value.Emit(), theSecret) {
				t.Fatalf("the attribute %s carries the secret", a.Key)
			}
		}
	}
}

func TestTheMessagingAndClientHelpersProduceSpans(t *testing.T) {
	p, _, exp := newRecorder(t)
	cases := []struct {
		name string
		kind trace.SpanKind
		run  func(context.Context) (context.Context, trace.Span)
	}{
		{"outbox.publish", trace.SpanKindProducer, func(ctx context.Context) (context.Context, trace.Span) {
			return p.StartProducer(ctx, "outbox.publish")
		}},
		{"inbox.handle", trace.SpanKindConsumer, func(ctx context.Context) (context.Context, trace.Span) {
			return p.StartConsumer(ctx, "inbox.handle")
		}},
		{"payments.charge", trace.SpanKindClient, func(ctx context.Context) (context.Context, trace.Span) {
			return p.StartClient(ctx, "payments.charge")
		}},
	}
	for _, tc := range cases {
		_, span := tc.run(context.Background())
		span.End()
		got := spanNamed(t, exp, tc.name)
		if got.SpanKind != tc.kind {
			t.Fatalf("%s has kind %v, want %v", tc.name, got.SpanKind, tc.kind)
		}
	}
}

func TestAChildSpanNestsUnderItsParent(t *testing.T) {
	p, _, exp := newRecorder(t)
	ctx, parent := p.StartConsumer(context.Background(), "inbox.handle")
	_, child := p.StartClient(ctx, "payments.charge")
	child.End()
	parent.End()

	outer := spanNamed(t, exp, "inbox.handle")
	inner := spanNamed(t, exp, "payments.charge")
	if inner.Parent.SpanID() != outer.SpanContext.SpanID() {
		t.Fatal("the client span does not nest under the consumer span")
	}
}

func TestTheNoOpPathAddsNoWrapper(t *testing.T) {
	// With every exporter off, the middleware must return the handler that it
	// received. A wrapper that does nothing still costs a call and an
	// allocation on every request.
	p, _ := newProvider(t)
	next := func(*router.Ctx) (router.Response, error) { return router.NoContent(), nil }
	got := p.HTTPMiddleware().Wrap(next)
	if fmt.Sprintf("%p", got) != fmt.Sprintf("%p", router.Handler(next)) {
		t.Fatal("the middleware wrapped the handler although every exporter is off")
	}
}

func TestTheNoOpPathAllocatesNothing(t *testing.T) {
	p, _ := newProvider(t)
	r := router.New()
	r.Use(p.HTTPMiddleware())
	r.Get("/x", func(*router.Ctx) (router.Response, error) { return router.NoContent(), nil })
	h, err := r.Handler()
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}

	plain := router.New()
	plain.Get("/x", func(*router.Ctx) (router.Response, error) { return router.NoContent(), nil })
	ph, err := plain.Handler()
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}

	measure := func(handler http.Handler) float64 {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		rec := httptest.NewRecorder()
		return testing.AllocsPerRun(200, func() {
			handler.ServeHTTP(rec, req)
		})
	}
	withTelemetry := measure(h)
	without := measure(ph)
	if withTelemetry > without {
		t.Fatalf("the no-op path added %v allocations for each request", withTelemetry-without)
	}
}

func TestTheEnabledPathStillServesTheRequest(t *testing.T) {
	p, _, _ := newRecorder(t)
	r := router.New()
	r.Use(p.HTTPMiddleware())
	r.Get("/x", func(*router.Ctx) (router.Response, error) { return router.Text(200, "done"), nil })
	if rec := serve(t, r, http.MethodGet, "/x"); rec.Code != 200 || rec.Body.String() != "done" {
		t.Fatalf("gave %d %q", rec.Code, rec.Body.String())
	}
}
