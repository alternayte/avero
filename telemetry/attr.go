package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/alternayte/avero/config"
)

// Attr builds a span attribute. A config.Secret reads as ******** and never
// reaches an exporter. See the SDD, S1 and S3.
func Attr(key string, value any) attribute.KeyValue {
	switch v := value.(type) {
	case config.Secret:
		return attribute.String(key, v.String())
	case string:
		return attribute.String(key, v)
	case bool:
		return attribute.Bool(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	default:
		return attribute.String(key, fmt.Sprint(v))
	}
}

// noSpan is the span that a disabled provider returns. It is a zero-size
// value, so it costs no allocation.
var noSpan = tracenoop.Span{}

// start begins a span of one kind, or returns the context unchanged when the
// exporter is off.
func (p *Provider) start(ctx context.Context, name string, kind trace.SpanKind,
	attrs ...attribute.KeyValue,
) (context.Context, trace.Span) {
	if !p.traces {
		return ctx, noSpan
	}
	return p.tracer.Start(ctx, name,
		trace.WithSpanKind(kind), trace.WithAttributes(attrs...))
}

// StartProducer begins a span for a message that the application publishes.
// The outbox relay calls it. See the SDD, S7.
func (p *Provider) StartProducer(ctx context.Context, name string,
	attrs ...attribute.KeyValue,
) (context.Context, trace.Span) {
	return p.start(ctx, name, trace.SpanKindProducer, attrs...)
}

// StartConsumer begins a span for a message that the application handles. The
// inbox consumer calls it. See the SDD, S8.
func (p *Provider) StartConsumer(ctx context.Context, name string,
	attrs ...attribute.KeyValue,
) (context.Context, trace.Span) {
	return p.start(ctx, name, trace.SpanKindConsumer, attrs...)
}

// StartClient begins a span for a call to another service. A generated client
// calls it. See the SDD, S13.
func (p *Provider) StartClient(ctx context.Context, name string,
	attrs ...attribute.KeyValue,
) (context.Context, trace.Span) {
	return p.start(ctx, name, trace.SpanKindClient, attrs...)
}

// StartInternal begins a span for work inside the application, such as a
// projection.
func (p *Provider) StartInternal(ctx context.Context, name string,
	attrs ...attribute.KeyValue,
) (context.Context, trace.Span) {
	return p.start(ctx, name, trace.SpanKindInternal, attrs...)
}
