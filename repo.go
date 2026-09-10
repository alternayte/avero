package avero

import (
	"context"
	"errors"
	"fmt"

	"github.com/alternayte/drel"
)

// errNoTx states the fault of a call that runs outside a transaction, and it
// states the repair. See DX-7.
var errNoTx = errors.New(
	"the call needs a transaction: register avero.Transaction in wire.go, or wrap the call in engine.WithTx")

// Repo returns the repository of the transaction of the request.
//
// The transaction middleware opens the transaction, so a handler always holds
// one. A job that reads outside a request opens its own with engine.WithTx.
// A read therefore reads its own writes, and a write stages the change on the
// transaction. See the SDD, S4.
//
//	repo, err := avero.Repo(ctx, model.PostMeta)
func Repo[T any](ctx context.Context, meta drel.ModelMeta[T]) (*drel.TxRepository[T], error) {
	tx, ok := drel.FromContext(ctx)
	if !ok {
		return nil, errNoTx
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
		return errNoTx
	}
	if err := tx.SaveChanges(ctx); err != nil {
		return fmt.Errorf("the change does not write: %w", err)
	}
	return nil
}
