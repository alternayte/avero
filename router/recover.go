package router

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover turns a panic into 500. It logs the value and the stack, and it
// never writes either to the client.
//
// Recover sits outside the transaction middleware in the scaffolded order, so
// the transaction rolls back before Recover answers. See the SDD, S4.
func Recover(log *slog.Logger) Middleware {
	return Middleware{Name: "recover", Wrap: func(next Handler) Handler {
		return func(c *Ctx) (res Response, err error) {
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				log.ErrorContext(c.Context(), "the handler panicked",
					"panic", fmt.Sprint(v),
					"method", c.Request().Method,
					"path", c.Request().URL.Path,
					"request_id", c.RequestID(),
					"stack", string(debug.Stack()))
				res, err = Status(http.StatusInternalServerError), nil
			}()
			return next(c)
		}
	}}
}
