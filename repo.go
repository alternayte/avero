package avero

import (
	"context"

	"github.com/alternayte/avero/db"
	"github.com/alternayte/drel"
)

// Repo returns the repository of the transaction of the request. See db.Repo.
//
// The transaction middleware opens the transaction, so a handler always holds
// one. A job that reads outside a request opens its own with engine.WithTx.
//
//	repo, err := avero.Repo(ctx, model.PostMeta)
func Repo[T any](ctx context.Context, meta drel.ModelMeta[T]) (*drel.TxRepository[T], error) {
	return db.Repo(ctx, meta)
}

// Save writes the staged changes inside the transaction of the request. See
// db.Save.
//
// The transaction still commits at the end of the request. The flush only
// makes a later read of the same request see the write.
func Save(ctx context.Context) error { return db.Save(ctx) }
