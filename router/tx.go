package router

import (
	"context"
	"errors"
	"fmt"

	"github.com/alternayte/drel"
)

// errRollback tells drel to roll back although the handler returned no error.
// It never reaches the caller.
var errRollback = errors.New("router: roll back this transaction")

// Transaction opens a drel transaction, puts it in the request context, and
// decides the commit from the response.
//
// It commits on a 2xx or a 3xx response. It rolls back on every other status,
// on a handler error and on a panic. See the SDD, S4.
//
// The handler returns a Response and writes nothing, so the commit happens
// before any byte reaches the client. A commit that fails therefore never
// follows a sent 200.
func Transaction(e *drel.Engine) Middleware {
	return Middleware{Name: "transaction", Wrap: func(next Handler) Handler {
		return func(c *Ctx) (Response, error) {
			var (
				res  Response
				herr error
			)
			txErr := e.WithTx(c.Context(), func(ctx context.Context) error {
				c.setRequest(c.Request().WithContext(ctx))
				res, herr = next(c)
				if herr != nil {
					return errRollback
				}
				if code := statusOf(res); code < 200 || code > 399 {
					return errRollback
				}
				return nil
			})
			switch {
			case herr != nil:
				// The handler failed. Its writes are rolled back. The router
				// turns the error into a response.
				return nil, herr
			case errors.Is(txErr, errRollback):
				// The status asked for the rollback. The response still goes
				// to the client.
				return res, nil
			case txErr != nil:
				// The transaction itself failed. Nothing was committed.
				return nil, fmt.Errorf("router: the transaction did not commit: %w", txErr)
			}
			return res, nil
		}
	}}
}
