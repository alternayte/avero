// Package db holds the drel glue of Avero: the transaction middleware, the
// transaction of a request and the repository of that transaction.
//
// The router imports no database. An application that keeps its own storage
// therefore imports the router and never compiles drel. An application that
// uses drel imports this package, and `avero.Stack` takes the middleware that
// Transaction returns.
//
// See the SDD, S4.
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/alternayte/avero/router"
	"github.com/alternayte/drel"
)

// errRollback tells drel to roll back although the handler returned no error.
// It never reaches the caller.
var errRollback = errors.New("db: roll back this transaction")

// ErrNoTx states the fault of a call that runs outside a transaction, and it
// states the repair. See DX-7.
var ErrNoTx = errors.New(
	"the call needs a transaction: put avero.Transaction in the Tx field of avero.Stack, or wrap the call in engine.WithTx")

// Transaction opens a drel transaction, puts it in the request context, and
// decides the commit from the response.
//
// It commits on a 2xx or a 3xx response. It rolls back on every other status,
// on a handler error and on a panic. See the SDD, S4.
//
// The handler returns a Response and writes nothing, so the commit happens
// before any byte reaches the client. A commit that fails therefore never
// follows a sent 200. NeedsResponse states that property, so a mounted
// handler, which writes for itself, runs no transaction.
//
// A nil engine returns the zero middleware, which Stack leaves out. An
// inspection command passes a nil engine, and the chain of the inspection
// stays the chain of a run in every other member.
//
//	r.Use(avero.Stack{Secret: cfg.Secret, Tx: db.Transaction(engine)}.Middleware()...)
func Transaction(e *drel.Engine) router.Middleware {
	if e == nil {
		return router.Middleware{}
	}
	return router.Middleware{Name: "transaction", NeedsResponse: true, Wrap: func(next router.Handler) router.Handler {
		return func(c *router.Ctx) (router.Response, error) {
			var (
				res  router.Response
				herr error
			)
			txErr := e.WithTx(c.Context(), func(ctx context.Context) error {
				c.SetContext(ctx)
				res, herr = next(c)
				if herr != nil {
					return errRollback
				}
				if code := router.StatusOf(res); code < 200 || code > 399 {
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
				return nil, fmt.Errorf("db: the transaction did not commit: %w", txErr)
			}
			return res, nil
		}
	}}
}

// Tx returns the transaction that Transaction opened, and reports whether one
// surrounds the request.
//
//	tx, ok := db.Tx(c)
func Tx(ctx context.Context) (*drel.Tx, bool) { return drel.FromContext(ctx) }

// MustTx returns the transaction and panics when none is present. Use it in a
// handler that a transaction must always surround, so that a wiring fault
// fails at once.
func MustTx(ctx context.Context) *drel.Tx { return drel.MustFromContext(ctx) }

// Repo returns the repository of the transaction of the request.
//
// The transaction middleware opens the transaction, so a handler always holds
// one. A job that reads outside a request opens its own with engine.WithTx.
// A read therefore reads its own writes, and a write stages the change on the
// transaction. See the SDD, S4.
//
//	repo, err := db.Repo(ctx, model.PostMeta)
func Repo[T any](ctx context.Context, meta drel.ModelMeta[T]) (*drel.TxRepository[T], error) {
	tx, ok := drel.FromContext(ctx)
	if !ok {
		return nil, ErrNoTx
	}
	return drel.NewTxRepository(tx, meta), nil
}

// Save writes the staged changes inside the transaction of the request.
//
// The transaction still commits at the end of the request. The flush only
// makes a later read of the same request see the write.
func Save(ctx context.Context) error {
	tx, ok := drel.FromContext(ctx)
	if !ok {
		return ErrNoTx
	}
	if err := tx.SaveChanges(ctx); err != nil {
		return fmt.Errorf("the change does not write: %w", err)
	}
	return nil
}
