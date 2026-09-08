package telemetry

import (
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/alternayte/avero/router"
)

// HTTPMiddleware fills the trace slot of the middleware chain. It starts one
// server span for each request and records the duration.
//
// With every exporter off it returns the handler that it received. The chain
// then holds no wrapper, so a request costs no extra call and no extra
// allocation. See the SDD, S3.
//
// The span carries the route pattern and never the raw path, the query, a
// header or a cookie. A secret in a URL or in a header therefore never reaches
// an exporter.
func (p *Provider) HTTPMiddleware() router.Middleware {
	if !p.traces && !p.metrics {
		return router.Middleware{
			Name: "trace",
			Wrap: func(next router.Handler) router.Handler { return next },
		}
	}
	return router.Middleware{Name: "trace", Wrap: func(next router.Handler) router.Handler {
		return func(c *router.Ctx) (router.Response, error) {
			method, route := splitPattern(c.Request().Pattern, c.Request().Method)
			start := time.Now()

			ctx, span := p.start(c.Context(), spanName(method, route), trace.SpanKindServer,
				semconv.HTTPRequestMethodKey.String(method),
				semconv.HTTPRoute(route))
			c.SetContext(ctx)

			res, err := next(c)
			code := statusOf(res, err)

			span.SetAttributes(semconv.HTTPResponseStatusCode(code))
			if err != nil {
				span.RecordError(err)
			}
			// A 5xx is a fault of the server. A 4xx is the answer of the
			// server to a wrong request, so it leaves the status unset.
			if code >= 500 {
				span.SetStatus(codes.Error, "")
			}
			span.End()

			if p.duration != nil {
				p.duration.Record(ctx, time.Since(start).Seconds(),
					metric.WithAttributes(
						semconv.HTTPRequestMethodKey.String(method),
						semconv.HTTPRoute(route),
						semconv.HTTPResponseStatusCode(code)))
			}
			return res, err
		}
	}}
}

// statusOf returns the status that the client will read.
func statusOf(res router.Response, err error) int {
	switch {
	case err != nil:
		return 500
	case res == nil:
		return 204
	}
	return res.Status()
}

// splitPattern cuts the matched pattern of net/http into the method and the
// route. A mounted handler carries no method in its pattern.
func splitPattern(pattern, method string) (string, string) {
	if pattern == "" {
		return method, ""
	}
	if m, route, ok := strings.Cut(pattern, " "); ok {
		return m, route
	}
	return method, pattern
}

// spanName names a server span after the route, so that the cardinality stays
// bounded. A path value never appears in it.
func spanName(method, route string) string {
	if route == "" {
		return method
	}
	return method + " " + route
}

var _ = attribute.String
