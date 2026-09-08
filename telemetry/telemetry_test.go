package telemetry_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/alternayte/avero/config"
	"github.com/alternayte/avero/host"
	"github.com/alternayte/avero/router"
	"github.com/alternayte/avero/telemetry"
)

func TestALogLineCarriesTheTraceIDOfItsRequest(t *testing.T) {
	p, buf, exp := newRecorder(t)
	log := p.Logger()

	r := router.New()
	r.Use(router.RequestID(), p.HTTPMiddleware())
	r.Get("/things/{id}", func(c *router.Ctx) (router.Response, error) {
		log.InfoContext(c.Context(), "inside the handler")
		return router.Text(200, "done"), nil
	})
	serve(t, r, http.MethodGet, "/things/7")

	span := spanNamed(t, exp, "GET /things/{id}")
	records := buf.records(t)
	if len(records) == 0 {
		t.Fatal("the handler wrote no log line")
	}
	last := records[len(records)-1]
	if last["trace_id"] != span.SpanContext.TraceID().String() {
		t.Fatalf("the log line carries trace_id %v, want %s",
			last["trace_id"], span.SpanContext.TraceID())
	}
	if last["span_id"] != span.SpanContext.SpanID().String() {
		t.Fatalf("the log line carries span_id %v, want %s",
			last["span_id"], span.SpanContext.SpanID())
	}
}

func TestALogLineCarriesTheRequestID(t *testing.T) {
	p, buf := newProvider(t)
	log := p.Logger()

	r := router.New()
	r.Use(router.RequestID(), p.HTTPMiddleware())
	r.Get("/x", func(c *router.Ctx) (router.Response, error) {
		log.InfoContext(c.Context(), "inside the handler")
		return router.Text(200, c.RequestID()), nil
	})
	rec := serve(t, r, http.MethodGet, "/x")

	records := buf.records(t)
	last := records[len(records)-1]
	if last["request_id"] != rec.Body.String() {
		t.Fatalf("the log line carries request_id %v, want %q", last["request_id"], rec.Body.String())
	}
}

func TestALogLineOutsideARequestCarriesNoIDs(t *testing.T) {
	p, buf := newProvider(t)
	p.Logger().Info("at boot")

	last := buf.records(t)[0]
	for _, key := range []string{"trace_id", "span_id", "request_id"} {
		if _, ok := last[key]; ok {
			t.Fatalf("a boot line carries %s: %v", key, last)
		}
	}
}

func TestTheLoggerKeepsTheIdentifiersAfterWith(t *testing.T) {
	p, buf, exp := newRecorder(t)
	log := p.Logger().With("component", "orders")

	r := router.New()
	r.Use(p.HTTPMiddleware())
	r.Get("/x", func(c *router.Ctx) (router.Response, error) {
		log.InfoContext(c.Context(), "inside the handler", "count", 3)
		return router.NoContent(), nil
	})
	serve(t, r, http.MethodGet, "/x")

	span := spanNamed(t, exp, "GET /x")
	last := buf.records(t)[0]
	if last["component"] != "orders" {
		t.Fatalf("the line drops the attribute: %v", last)
	}
	if last["count"] == nil {
		t.Fatalf("the line drops the call attribute: %v", last)
	}
	if last["trace_id"] != span.SpanContext.TraceID().String() {
		t.Fatalf("the line drops the trace id after With: %v", last)
	}
}

func TestAnOpenGroupNestsTheIdentifiers(t *testing.T) {
	// slog gives a handler no way to place an attribute outside a group that
	// the caller opened. The identifiers therefore sit inside the group. Open
	// a group on a child logger and not on the logger of the application.
	p, buf, exp := newRecorder(t)
	log := p.Logger().WithGroup("detail")

	r := router.New()
	r.Use(p.HTTPMiddleware())
	r.Get("/x", func(c *router.Ctx) (router.Response, error) {
		log.InfoContext(c.Context(), "inside the handler", "count", 3)
		return router.NoContent(), nil
	})
	serve(t, r, http.MethodGet, "/x")

	span := spanNamed(t, exp, "GET /x")
	last := buf.records(t)[0]
	detail, ok := last["detail"].(map[string]any)
	if !ok {
		t.Fatalf("the line drops the group: %v", last)
	}
	if detail["count"] == nil {
		t.Fatalf("the group drops the call attribute: %v", last)
	}
	if detail["trace_id"] != span.SpanContext.TraceID().String() {
		t.Fatalf("the group drops the trace id: %v", last)
	}
	if _, ok := last["trace_id"]; ok {
		t.Fatalf("the identifiers are at the top level and inside the group: %v", last)
	}
}

func TestTheLoggerReadsLogLevel(t *testing.T) {
	buf := &syncBuffer{}
	cfg := baseConfig()
	cfg.LogLevel = "warn"
	p, err := telemetry.New(cfg, telemetry.WithLogWriter(buf))
	if err != nil {
		t.Fatalf("New returned an error: %v", err)
	}
	p.Logger().Info("this is below the level")
	p.Logger().Warn("this is at the level")
	if len(buf.records(t)) != 1 {
		t.Fatalf("the logger wrote %d lines, want 1:\n%s", len(buf.records(t)), buf.String())
	}
}

func TestTheLoggerReadsLogFormat(t *testing.T) {
	buf := &syncBuffer{}
	cfg := baseConfig()
	cfg.LogFormat = "text"
	p, err := telemetry.New(cfg, telemetry.WithLogWriter(buf))
	if err != nil {
		t.Fatalf("New returned an error: %v", err)
	}
	p.Logger().Info("a line")
	if !strings.Contains(buf.String(), "msg=\"a line\"") {
		t.Fatalf("the format is not text:\n%s", buf.String())
	}
}

func TestTheDefaultExporterIsNoOp(t *testing.T) {
	p, _ := newProvider(t)
	if p.TracesEnabled() {
		t.Fatal("traces are enabled with OTEL_TRACES_EXPORTER=none")
	}
	if p.MetricsEnabled() {
		t.Fatal("metrics are enabled with OTEL_METRICS_EXPORTER=none")
	}
	if p.DrelTracer() != nil {
		t.Fatal("DrelTracer returned a tracer with the exporter off")
	}
}

func TestAnUnknownExporterIsAFault(t *testing.T) {
	cfg := baseConfig()
	cfg.OTel.TracesExporter = "jaeger"
	_, err := telemetry.New(cfg)
	if err == nil {
		t.Fatal("New accepted an unknown trace exporter")
	}
	if !strings.Contains(err.Error(), "OTEL_TRACES_EXPORTER") || !strings.Contains(err.Error(), "→") {
		t.Fatalf("the fault does not name the variable or state the repair: %v", err)
	}
}

func TestTheGRPCProtocolIsAFault(t *testing.T) {
	// Avero ships the http/protobuf exporter only. gRPC is a dependency that
	// the SDD does not name.
	cfg := baseConfig()
	cfg.OTel.TracesExporter = "otlp"
	cfg.OTel.ExporterOTLPProtocol = "grpc"
	_, err := telemetry.New(cfg)
	if err == nil {
		t.Fatal("New accepted the grpc protocol")
	}
	if !strings.Contains(err.Error(), "OTEL_EXPORTER_OTLP_PROTOCOL") {
		t.Fatalf("the fault does not name the variable: %v", err)
	}
	if !strings.Contains(err.Error(), "http/protobuf") {
		t.Fatalf("the repair does not name the protocol that works: %v", err)
	}
}

func TestTheProviderIsAComponent(t *testing.T) {
	p, _ := newProvider(t)
	var c host.Component = p
	if c.Name() == "" {
		t.Fatal("the component has no name")
	}
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start returned an error: %v", err)
	}
	if err := c.Stop(context.Background()); err != nil {
		t.Fatalf("Stop returned an error: %v", err)
	}
}

func TestStopIsSafeToCallTwoTimes(t *testing.T) {
	p, _ := newProvider(t)
	if err := p.Stop(context.Background()); err != nil {
		t.Fatalf("the first Stop returned an error: %v", err)
	}
	if err := p.Stop(context.Background()); err != nil {
		t.Fatalf("the second Stop returned an error: %v", err)
	}
}

func TestTheProviderSetsNoGlobalProvider(t *testing.T) {
	// Design rule 1 forbids global mutable state. The application passes the
	// provider to each subsystem.
	before := otelGlobalTracerProvider()
	_, _, _ = newRecorder(t)
	if otelGlobalTracerProvider() != before {
		t.Fatal("New replaced the global tracer provider")
	}
}

var _ = config.BaseConfig{}
