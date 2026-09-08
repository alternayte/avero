// Package telemetry wires OpenTelemetry traces and metrics, and log/slog.
//
// The provider is a value. It sets no global provider, because design rule 1
// forbids global mutable state. The application passes it to each subsystem
// that produces a span.
//
// The default exporter is none. With every exporter off, the HTTP middleware
// returns the handler that it received, so a request costs no extra
// allocation. See the SDD, S3.
package telemetry

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/alternayte/avero/config"
)

// The exporter names that OTEL_TRACES_EXPORTER and OTEL_METRICS_EXPORTER take.
const (
	// ExporterNone produces no telemetry. It is the default.
	ExporterNone = "none"
	// ExporterOTLP sends to a collector over OTLP.
	ExporterOTLP = "otlp"
)

// ProtocolHTTP is the only OTLP protocol that Avero ships.
//
// The gRPC exporter is a module that the SDD does not name, so Avero does not
// import it. google.golang.org/grpc still reaches the build through
// go.opentelemetry.io/proto/otlp, which the OTLP HTTP exporters use for their
// message types. Avero opens no gRPC connection.
const ProtocolHTTP = "http/protobuf"

// scope names the instrumentation library in every span and metric.
const scope = "github.com/alternayte/avero"

// Provider holds the tracer, the meter and the logger of one application.
// Build it with New and register it as a component, so that Stop flushes the
// exporters inside SHUTDOWN_GRACE.
type Provider struct {
	cfg    config.BaseConfig
	log    *slog.Logger
	tracer trace.Tracer
	meter  metric.Meter

	tp *sdktrace.TracerProvider
	mp *sdkmetric.MeterProvider

	traces  bool
	metrics bool

	duration metric.Float64Histogram

	stopOnce sync.Once
	stopErr  error
}

// Option configures a provider.
type Option func(*options)

type options struct {
	logWriter io.Writer
	tp        trace.TracerProvider
	mp        metric.MeterProvider
}

// WithLogWriter sends the log to w. The default is standard error.
func WithLogWriter(w io.Writer) Option { return func(o *options) { o.logWriter = w } }

// WithTracerProvider supplies the tracer provider. A test passes one that
// keeps every span in memory. It replaces the exporter that the configuration
// names.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(o *options) { o.tp = tp }
}

// WithMeterProvider supplies the meter provider. It replaces the exporter that
// the configuration names.
func WithMeterProvider(mp metric.MeterProvider) Option {
	return func(o *options) { o.mp = mp }
}

// New builds a provider from the OTEL_* variables of the configuration.
func New(cfg config.BaseConfig, opts ...Option) (*Provider, error) {
	o := &options{logWriter: os.Stderr}
	for _, opt := range opts {
		opt(o)
	}

	p := &Provider{cfg: cfg}
	p.log = newLogger(cfg, o.logWriter)

	if err := checkExporter("OTEL_TRACES_EXPORTER", cfg.OTel.TracesExporter); err != nil {
		return nil, err
	}
	if err := checkExporter("OTEL_METRICS_EXPORTER", cfg.OTel.MetricsExporter); err != nil {
		return nil, err
	}

	// A supplied provider replaces the exporter of the configuration, so a
	// test records spans without a collector.
	switch {
	case o.tp != nil:
		p.traces = true
		p.tracer = o.tp.Tracer(scope)
	case cfg.OTel.TracesExporter == ExporterOTLP:
		if err := checkProtocol(cfg); err != nil {
			return nil, err
		}
		exporter, err := otlptracehttp.New(context.Background(),
			otlptracehttp.WithEndpointURL(cfg.OTel.ExporterOTLPEndpoint.String()))
		if err != nil {
			return nil, &Fault{
				Variable: "OTEL_EXPORTER_OTLP_ENDPOINT",
				Message:  "the trace exporter did not open",
				Repair:   "Set OTEL_EXPORTER_OTLP_ENDPOINT to the address of the collector, or set OTEL_TRACES_EXPORTER=none",
				Err:      err,
			}
		}
		p.tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(newResource(cfg)))
		p.traces = true
		p.tracer = p.tp.Tracer(scope)
	default:
		p.tracer = tracenoop.NewTracerProvider().Tracer(scope)
	}

	switch {
	case o.mp != nil:
		p.metrics = true
		p.meter = o.mp.Meter(scope)
	case cfg.OTel.MetricsExporter == ExporterOTLP:
		if err := checkProtocol(cfg); err != nil {
			return nil, err
		}
		exporter, err := otlpmetrichttp.New(context.Background(),
			otlpmetrichttp.WithEndpointURL(cfg.OTel.ExporterOTLPEndpoint.String()))
		if err != nil {
			return nil, &Fault{
				Variable: "OTEL_EXPORTER_OTLP_ENDPOINT",
				Message:  "the metric exporter did not open",
				Repair:   "Set OTEL_EXPORTER_OTLP_ENDPOINT to the address of the collector, or set OTEL_METRICS_EXPORTER=none",
				Err:      err,
			}
		}
		p.mp = sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
			sdkmetric.WithResource(newResource(cfg)))
		p.metrics = true
		p.meter = p.mp.Meter(scope)
	default:
		p.meter = metricnoop.NewMeterProvider().Meter(scope)
	}

	if p.metrics {
		d, err := p.meter.Float64Histogram("http.server.request.duration",
			metric.WithDescription("the duration of one HTTP request"),
			metric.WithUnit("s"))
		if err != nil {
			return nil, fmt.Errorf("telemetry: the request histogram did not build: %w", err)
		}
		p.duration = d
	}
	return p, nil
}

// Name identifies the component. See the SDD, section 5.1.
func (p *Provider) Name() string { return "telemetry" }

// Start does nothing. New builds the exporters, so a fault appears before the
// process starts. See DX-8.
func (p *Provider) Start(context.Context) error { return nil }

// Stop flushes the exporters. The context carries the SHUTDOWN_GRACE deadline.
// It is safe to call more than one time.
func (p *Provider) Stop(ctx context.Context) error {
	p.stopOnce.Do(func() {
		if p.tp != nil {
			if err := p.tp.Shutdown(ctx); err != nil {
				p.stopErr = fmt.Errorf("telemetry: the trace exporter did not flush: %w", err)
			}
		}
		if p.mp != nil {
			if err := p.mp.Shutdown(ctx); err != nil && p.stopErr == nil {
				p.stopErr = fmt.Errorf("telemetry: the metric exporter did not flush: %w", err)
			}
		}
	})
	return p.stopErr
}

// Logger returns the logger. Every record inside a request carries the trace
// ID, the span ID and the request ID.
func (p *Provider) Logger() *slog.Logger { return p.log }

// Tracer returns the tracer. It is a no-op tracer when the exporter is none.
func (p *Provider) Tracer() trace.Tracer { return p.tracer }

// Meter returns the meter. It is a no-op meter when the exporter is none.
func (p *Provider) Meter() metric.Meter { return p.meter }

// TracesEnabled reports whether a span reaches an exporter.
func (p *Provider) TracesEnabled() bool { return p.traces }

// MetricsEnabled reports whether a measurement reaches an exporter.
func (p *Provider) MetricsEnabled() bool { return p.metrics }

// newResource describes the service in every span and metric. An empty service
// name leaves the SDK default in place, as the OpenTelemetry specification
// states.
func newResource(cfg config.BaseConfig) *resource.Resource {
	attrs := []attribute.KeyValue{
		semconv.DeploymentEnvironment(cfg.Env),
	}
	if cfg.OTel.ServiceName != "" {
		attrs = append(attrs, semconv.ServiceName(cfg.OTel.ServiceName))
	}
	r, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, attrs...))
	if err != nil {
		// The merge fails only on a schema mismatch, and both sides carry the
		// same schema. Fall back to the attributes alone.
		return resource.NewWithAttributes(semconv.SchemaURL, attrs...)
	}
	return r
}

// checkExporter refuses an exporter name that Avero does not ship.
func checkExporter(variable, name string) error {
	switch name {
	case ExporterNone, ExporterOTLP, "":
		return nil
	}
	return &Fault{
		Variable: variable,
		Message:  fmt.Sprintf("%s names the exporter %q, and Avero ships none and otlp", variable, name),
		Repair:   fmt.Sprintf("Set %s to otlp or to none", variable),
	}
}

// checkProtocol refuses a protocol that Avero does not ship. gRPC needs a
// dependency that the SDD does not name.
func checkProtocol(cfg config.BaseConfig) error {
	p := cfg.OTel.ExporterOTLPProtocol
	if p == ProtocolHTTP || p == "" {
		return nil
	}
	return &Fault{
		Variable: "OTEL_EXPORTER_OTLP_PROTOCOL",
		Message:  fmt.Sprintf("OTEL_EXPORTER_OTLP_PROTOCOL names %q, and Avero ships http/protobuf only", p),
		Repair:   "Set OTEL_EXPORTER_OTLP_PROTOCOL to http/protobuf, and point the endpoint at the OTLP HTTP port of the collector",
	}
}
