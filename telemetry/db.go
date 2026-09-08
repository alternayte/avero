package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/trace"

	"github.com/alternayte/drel"
)

// drelTracer bridges the drel tracing contract to OpenTelemetry. drel names
// the span, and the name is a fixed string such as drel.exec, so no SQL and no
// argument reaches an exporter.
type drelTracer struct{ tracer trace.Tracer }

// Start begins a query span. The context that drel passes carries the request
// span, so the query span nests under it.
func (t drelTracer) Start(ctx context.Context, name string) (context.Context, drel.Span) {
	ctx, span := t.tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindClient))
	return ctx, drelSpan{span: span}
}

// drelSpan ends one query span.
type drelSpan struct{ span trace.Span }

// End closes the span.
func (s drelSpan) End() { s.span.End() }

// RecordError attaches the fault to the span. drel ignores a nil error.
func (s drelSpan) RecordError(err error) {
	if err == nil {
		return
	}
	s.span.RecordError(err)
}

// DrelTracer returns the tracer that drel uses. It returns nil when the
// exporter is off, and drel then produces no span.
func (p *Provider) DrelTracer() drel.Tracer {
	if !p.traces {
		return nil
	}
	return drelTracer{tracer: p.tracer}
}

// DrelOption returns the engine option that installs the tracer.
//
//	engine, err := drel.NewEngine(cfg.DatabaseURL, telemetry.DrelOption())
func (p *Provider) DrelOption() drel.Option {
	return drel.WithTracer(p.DrelTracer())
}
