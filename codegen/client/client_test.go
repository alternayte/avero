package client_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alternayte/avero/codegen/client"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// newClient builds a client against a test server, with no delay between the
// attempts.
func newClient(t *testing.T, h http.Handler, opts ...client.Option) *client.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := client.New("Example", append([]client.Option{
		client.WithBaseURL(srv.URL),
		client.WithBackoff(time.Millisecond),
	}, opts...)...)
	if err != nil {
		t.Fatalf("New returned %v, want nil", err)
	}
	return c
}

func TestNewNamesTheRepairForAnAbsentBase(t *testing.T) {
	if _, err := client.New("Example"); err == nil {
		t.Fatal("New returned nil, want a fault")
	}
	_, err := client.New("Example", client.WithBaseURL("api.example.com"))
	if err == nil || !strings.Contains(err.Error(), "→") {
		t.Fatalf("New returned %v, want a fault with a repair", err)
	}
}

func TestDoSendsTheHeadersTheQueryAndTheBody(t *testing.T) {
	type in struct {
		Title string `json:"title"`
	}
	var (
		gotPath, gotQuery, gotKey, gotAuth, gotBody, gotType string
	)
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		gotKey, gotAuth = r.Header.Get("X-Api-Key"), r.Header.Get("Authorization")
		gotType, gotBody = r.Header.Get("Content-Type"), string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"7"}`)
	}), client.WithAuth(client.AuthBearer), client.WithToken("secret"))

	var out struct {
		ID string `json:"id"`
	}
	req := client.Request{
		Op: "CreateThing", Method: http.MethodPost, Path: "/things/7",
		Query:  map[string][]string{"page": {"2"}},
		Header: http.Header{"X-Api-Key": {"k"}},
		Body:   in{Title: "a"},
	}
	if err := c.Do(context.Background(), req, &out); err != nil {
		t.Fatalf("Do returned %v, want nil", err)
	}
	if gotPath != "/things/7" || gotQuery != "page=2" {
		t.Fatalf("the server read %q %q", gotPath, gotQuery)
	}
	if gotKey != "k" || gotAuth != "Bearer secret" {
		t.Fatalf("the server read %q %q", gotKey, gotAuth)
	}
	if gotType != "application/json" || gotBody != `{"title":"a"}` {
		t.Fatalf("the server read %q %q", gotType, gotBody)
	}
	if out.ID != "7" {
		t.Fatalf("ID = %q, want 7", out.ID)
	}
}

func TestAFourOhFourMapsToATypedErrorThatKeepsTheResponse(t *testing.T) {
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "abc")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"absent"}`)
	}))
	err := c.Do(context.Background(), client.Request{Op: "GetThing", Method: http.MethodGet, Path: "/things/7", Idempotent: true}, nil)
	if err == nil {
		t.Fatal("Do returned nil, want a fault")
	}
	var status *client.StatusError
	if !errors.As(err, &status) {
		t.Fatalf("the error is %T, want *client.StatusError", err)
	}
	if status.Status != http.StatusNotFound {
		t.Fatalf("Status = %d, want 404", status.Status)
	}
	if status.Response == nil || status.Response.Header.Get("X-Request-Id") != "abc" {
		t.Fatal("the error does not reach the response")
	}
	body, readErr := io.ReadAll(status.Response.Body)
	if readErr != nil || string(body) != `{"error":"absent"}` {
		t.Fatalf("the response body reads %q, %v", body, readErr)
	}
	if string(status.Body) != `{"error":"absent"}` {
		t.Fatalf("Body holds %q", status.Body)
	}
}

func TestAFourOhFourDoesNotRetry(t *testing.T) {
	var calls atomic.Int32
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	_ = c.Do(context.Background(), client.Request{Op: "GetThing", Method: http.MethodGet, Path: "/x", Idempotent: true}, nil)
	if calls.Load() != 1 {
		t.Fatalf("the server answered %d calls, want 1", calls.Load())
	}
}

func TestAFiveHundredRetriesAndThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, `{"id":"7"}`)
	}))
	var out struct {
		ID string `json:"id"`
	}
	if err := c.Do(context.Background(), client.Request{Op: "GetThing", Method: http.MethodGet, Path: "/x", Idempotent: true}, &out); err != nil {
		t.Fatalf("Do returned %v, want nil", err)
	}
	if calls.Load() != 3 {
		t.Fatalf("the server answered %d calls, want 3", calls.Load())
	}
	if out.ID != "7" {
		t.Fatalf("ID = %q", out.ID)
	}
}

func TestANonIdempotentCallDoesNotRetry(t *testing.T) {
	var calls atomic.Int32
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	err := c.Do(context.Background(), client.Request{Op: "CreateThing", Method: http.MethodPost, Path: "/x"}, nil)
	if err == nil {
		t.Fatal("Do returned nil, want a fault")
	}
	if calls.Load() != 1 {
		t.Fatalf("the server answered %d calls, want 1", calls.Load())
	}
}

func TestTheAttemptsStopAtTheLimit(t *testing.T) {
	var calls atomic.Int32
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}), client.WithAttempts(2))
	err := c.Do(context.Background(), client.Request{Op: "GetThing", Method: http.MethodGet, Path: "/x", Idempotent: true}, nil)
	var status *client.StatusError
	if !errors.As(err, &status) || status.Status != http.StatusServiceUnavailable {
		t.Fatalf("Do returned %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("the server answered %d calls, want 2", calls.Load())
	}
}

func TestTheBreakerOpensAndFailsTheNextCallAtOnce(t *testing.T) {
	var calls atomic.Int32
	breaker := client.NewBreaker(client.BreakerConfig{Threshold: 2, Cooldown: time.Hour})
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}), client.WithBreaker(breaker), client.WithAttempts(1))

	req := client.Request{Op: "GetThing", Method: http.MethodGet, Path: "/x", Idempotent: true}
	_ = c.Do(context.Background(), req, nil)
	_ = c.Do(context.Background(), req, nil)
	if breaker.State() != client.StateOpen {
		t.Fatalf("the breaker is %s, want open", breaker.State())
	}
	err := c.Do(context.Background(), req, nil)
	if !errors.Is(err, client.ErrBreakerOpen) {
		t.Fatalf("Do returned %v, want ErrBreakerOpen", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("the server answered %d calls, want 2", calls.Load())
	}
}

func TestADeadlineReachesTheCaller(t *testing.T) {
	c := newClient(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}), client.WithTimeout(20*time.Millisecond), client.WithAttempts(1))
	err := c.Do(context.Background(), client.Request{Op: "GetThing", Method: http.MethodGet, Path: "/x", Idempotent: true}, nil)
	var transport *client.TransportError
	if !errors.As(err, &transport) {
		t.Fatalf("the error is %T, want *client.TransportError", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("errors.Is does not reach the deadline: %v", err)
	}
}

func TestEachCallRecordsASpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	}), client.WithTracer(provider.Tracer("test")))

	if err := c.Do(context.Background(), client.Request{
		Op: "GetThing", Method: http.MethodGet, Path: "/things/7", Idempotent: true,
	}, nil); err != nil {
		t.Fatalf("Do returned %v, want nil", err)
	}
	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("the call recorded %d spans, want 1", len(spans))
	}
	span := spans[0]
	if span.Name() != "GET Example.GetThing" {
		t.Fatalf("the span is %q", span.Name())
	}
	if span.SpanKind() != trace.SpanKindClient {
		t.Fatalf("the kind is %v, want a client span", span.SpanKind())
	}
	found := map[string]string{}
	for _, a := range span.Attributes() {
		found[string(a.Key)] = a.Value.Emit()
	}
	if found["http.request.method"] != "GET" || found["http.response.status_code"] != "200" {
		t.Fatalf("the attributes are %v", found)
	}
	if !strings.Contains(found["url.full"], "/things/7") {
		t.Fatalf("url.full is %q", found["url.full"])
	}
}

func TestAFailedCallMarksTheSpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}), client.WithTracer(provider.Tracer("test")))

	_ = c.Do(context.Background(), client.Request{Op: "GetThing", Method: http.MethodGet, Path: "/x", Idempotent: true}, nil)
	spans := recorder.Ended()
	if len(spans) != 1 || spans[0].Status().Code != codes.Error {
		t.Fatalf("the span is %+v", spans)
	}
}
