package config

import (
	"net/url"
	"time"
)

// BaseConfig holds the variables that every Avero application reads. Embed it
// in the application configuration struct. An embedded struct takes no prefix,
// so PORT stays PORT.
type BaseConfig struct {
	// Port is the port that the HTTP server binds.
	Port int `env:"PORT" default:"8080"`
	// LogLevel is one of debug, info, warn or error.
	LogLevel string `env:"LOG_LEVEL" default:"info"`
	// LogFormat is json or text.
	LogFormat string `env:"LOG_FORMAT" default:"json"`
	// ShutdownGrace is the deadline that Stop receives. See the SDD, S2.
	ShutdownGrace time.Duration `env:"SHUTDOWN_GRACE" default:"15s"`
	// MigrateOnBoot applies pending migrations at start. See the SDD, 5.3.
	MigrateOnBoot bool `env:"MIGRATE_ON_BOOT" default:"false"`
	// Env is the deployment environment, such as development or production.
	Env string `env:"AVERO_ENV" default:"development"`
	// Secret signs the cookies that Avero owns: the CSRF token and the flash
	// that carries a toast across a redirect. auth-all owns the
	// authentication session and holds its own secret.
	//
	// It is required, so a missing secret stops the process before it serves.
	// See DX-8. Use at least 32 bytes, for example the output of
	// `openssl rand -hex 32`. Call Reveal to read the value.
	Secret Secret `env:"AVERO_SECRET,required,secret"`
	// OTel holds the standard OpenTelemetry variables.
	OTel OTelConfig `env:"OTEL"`
}

// OTelConfig holds the standard OTEL_* variables. The default exporter is
// none, so an application produces no telemetry until a person asks for it.
// See the SDD, S3.
type OTelConfig struct {
	// ServiceName names the service in a trace and in a metric.
	ServiceName string `env:"SERVICE_NAME" default:""`
	// ExporterOTLPEndpoint is the collector address.
	ExporterOTLPEndpoint url.URL `env:"EXPORTER_OTLP_ENDPOINT" default:""`
	// ExporterOTLPProtocol is grpc or http/protobuf.
	ExporterOTLPProtocol string `env:"EXPORTER_OTLP_PROTOCOL" default:"http/protobuf"`
	// TracesExporter is otlp or none.
	TracesExporter string `env:"TRACES_EXPORTER" default:"none"`
	// MetricsExporter is otlp or none.
	MetricsExporter string `env:"METRICS_EXPORTER" default:"none"`
}
