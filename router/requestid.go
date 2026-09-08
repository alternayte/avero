package router

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// HeaderRequestID names the header that carries the request ID.
const HeaderRequestID = "X-Request-Id"

type requestIDKey struct{}

// RequestID gives each request an identifier. It keeps the identifier that the
// client sent, so that a trace crosses a service boundary. It writes the
// identifier to the response header.
//
// S3 puts the same identifier on every log line inside the request.
func RequestID() Middleware {
	return Middleware{Name: "request_id", Wrap: func(next Handler) Handler {
		return func(c *Ctx) (Response, error) {
			id := c.Request().Header.Get(HeaderRequestID)
			if id == "" {
				id = newID()
			}
			c.Header().Set(HeaderRequestID, id)
			c.setRequest(c.Request().WithContext(
				context.WithValue(c.Context(), requestIDKey{}, id)))
			return next(c)
		}
	}}
}

// RequestIDFrom returns the identifier that RequestID put in ctx. It returns
// the empty string when the middleware did not run.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// RequestID returns the identifier of this request.
func (c *Ctx) RequestID() string { return RequestIDFrom(c.Context()) }

// newID returns a random identifier.
func newID() string {
	var b [16]byte
	// rand.Read fills b or it panics. It never returns a short read.
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// statusOf returns the status that a response will send.
func statusOf(res Response) int {
	if res == nil {
		return http.StatusNoContent
	}
	return res.Status()
}
