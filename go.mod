module github.com/alternayte/avero

go 1.26.2

// github.com/alternayte/drel is a direct dependency of the router package. It
// is absent here because drel does not publish yet: its module path names the
// repository github.com/alternayte/drel, which does not exist, and no tag
// carries the context transaction API. go.work supplies the local checkout.
//
// Add the require and run `go mod tidy` when drel publishes a version. See
// the note inside go.work.

// The direct dependencies. Every one is named in the SDD: auth-all in
// section 1.1, and OpenTelemetry in S3.
require (
	github.com/alternayte/auth-all v0.2.0
	go.opentelemetry.io/otel v1.41.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.41.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.41.0
	go.opentelemetry.io/otel/metric v1.41.0
	go.opentelemetry.io/otel/sdk v1.41.0
	go.opentelemetry.io/otel/sdk/metric v1.41.0
	go.opentelemetry.io/otel/trace v1.41.0
)

// google.golang.org/grpc arrives through go.opentelemetry.io/proto/otlp, which
// the OTLP HTTP exporters use for their message types. Avero opens no gRPC
// connection. See the note on ProtocolHTTP in the telemetry package.
require (
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.28.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.41.0 // indirect
	go.opentelemetry.io/proto/otlp v1.9.0 // indirect
	golang.org/x/crypto v0.55.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260209200024-4cfbd4190f57 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260209200024-4cfbd4190f57 // indirect
	google.golang.org/grpc v1.79.1 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
