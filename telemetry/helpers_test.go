package telemetry_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/alternayte/avero/config"
	"github.com/alternayte/avero/router"
	"github.com/alternayte/avero/telemetry"
	"github.com/alternayte/drel"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// theSecret is the value that must never reach a span attribute or a log line.
const theSecret = "sk-live-never-in-a-span"

// baseConfig returns a configuration with the exporters turned off.
func baseConfig() config.BaseConfig {
	return config.BaseConfig{
		LogLevel:  "debug",
		LogFormat: "json",
		Secret:    config.Secret(strings.Repeat("k", 32)),
		OTel: config.OTelConfig{
			ServiceName:     "orders",
			TracesExporter:  "none",
			MetricsExporter: "none",
		},
	}
}

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

// records returns each log line of the buffer as a map.
func (s *syncBuffer) records(t *testing.T) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(s.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("the log line does not parse as JSON: %q", line)
		}
		out = append(out, m)
	}
	return out
}

// newProvider builds a provider with a log buffer and an optional tracer.
func newProvider(t *testing.T, opts ...telemetry.Option) (*telemetry.Provider, *syncBuffer) {
	t.Helper()
	buf := &syncBuffer{}
	p, err := telemetry.New(baseConfig(), append([]telemetry.Option{
		telemetry.WithLogWriter(buf),
	}, opts...)...)
	if err != nil {
		t.Fatalf("New returned an error: %v", err)
	}
	t.Cleanup(func() { _ = p.Stop(context.Background()) })
	return p, buf
}

// newRecorder builds a provider that keeps every span in memory.
func newRecorder(t *testing.T) (*telemetry.Provider, *syncBuffer, *tracetest.InMemoryExporter) {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	p, buf := newProvider(t, telemetry.WithTracerProvider(tp))
	return p, buf, exp
}

// newEngine returns a drel engine with a table, traced by the provider.
func newEngine(t *testing.T, p *telemetry.Provider) *drel.Engine {
	t.Helper()
	e, err := drel.NewEngine(filepath.Join(t.TempDir(), "test.db"), p.DrelOption())
	if err != nil {
		t.Fatalf("NewEngine returned an error: %v", err)
	}
	t.Cleanup(e.Close)
	err = e.WithTx(context.Background(), func(ctx context.Context) error {
		_, execErr := drel.MustFromContext(ctx).Exec(ctx, `CREATE TABLE things (name text)`)
		return execErr
	})
	if err != nil {
		t.Fatalf("the schema did not apply: %v", err)
	}
	return e
}

// serve builds the router and performs one request.
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

// spanNamed returns the recorded span with this name.
func spanNamed(t *testing.T, exp *tracetest.InMemoryExporter, name string) tracetest.SpanStub {
	t.Helper()
	for _, s := range exp.GetSpans() {
		if s.Name == name {
			return s
		}
	}
	var names []string
	for _, s := range exp.GetSpans() {
		names = append(names, s.Name)
	}
	t.Fatalf("no span is named %q; the exporter holds %v", name, names)
	return tracetest.SpanStub{}
}

var _ = http.MethodGet

// otelGlobalTracerProvider returns the global provider, so that a test proves
// that Avero never sets it.
func otelGlobalTracerProvider() any { return otel.GetTracerProvider() }
