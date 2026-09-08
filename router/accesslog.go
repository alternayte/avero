package router

import (
	"log/slog"
	"time"
)

// AccessLog writes one line for each request. It records the method, the path,
// the status, the duration and the request ID.
//
// S3 adds the trace ID to the same line.
func AccessLog(log *slog.Logger) Middleware {
	return Middleware{Name: "access_log", Wrap: func(next Handler) Handler {
		return func(c *Ctx) (Response, error) {
			start := time.Now()
			res, err := next(c)
			log.InfoContext(c.Context(), "request",
				"method", c.Request().Method,
				"path", c.Request().URL.Path,
				"status", statusOf(res),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", c.RequestID())
			return res, err
		}
	}}
}
