package telemetry

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel/trace"

	"github.com/alternayte/avero/config"
	"github.com/alternayte/avero/router"
)

// newLogger builds the logger that the application uses. It reads LOG_LEVEL
// and LOG_FORMAT, and it wraps the handler so that every record inside a
// request carries the trace ID, the span ID and the request ID.
func newLogger(cfg config.BaseConfig, w io.Writer) *slog.Logger {
	level := slog.LevelInfo
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}

	var h slog.Handler
	if cfg.LogFormat == "text" {
		h = slog.NewTextHandler(w, opts)
	} else {
		h = slog.NewJSONHandler(w, opts)
	}
	return slog.New(contextHandler{Handler: h})
}

// contextHandler adds the identifiers of the request to each record. A record
// outside a request carries none of them, and the handler then adds nothing.
type contextHandler struct{ slog.Handler }

// Handle adds trace_id, span_id and request_id when the context carries them.
func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()))
	}
	if id := router.RequestIDFrom(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs keeps the wrapper, so that With does not drop the identifiers.
func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup keeps the wrapper, so that a group does not drop the identifiers.
//
// The identifiers then sit inside the group. slog gives a handler no way to
// place an attribute outside a group that the caller opened, and dropping the
// wrapper instead would lose the identifiers. Open a group on a child logger
// and not on the logger of the application.
func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{Handler: h.Handler.WithGroup(name)}
}
