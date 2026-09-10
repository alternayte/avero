package avero_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/drel"

	avero "github.com/alternayte/avero"
)

// row is a model that no generator wrote. Repo needs the meta value only to
// build the repository, so a zero meta proves the lookup of the transaction.
type row struct{}

// A call outside a transaction names the repair, so a person reads what to
// register in wire.go.
func TestRepoWithoutATransaction(t *testing.T) {
	_, err := avero.Repo(context.Background(), drel.ModelMeta[row]{})
	if err == nil {
		t.Fatal("Repo returned no error outside a transaction")
	}
	if !strings.Contains(err.Error(), "avero.Transaction") {
		t.Fatalf("the error does not state the repair: %v", err)
	}
}

// A call inside a transaction returns the repository of that transaction.
func TestRepoInsideATransaction(t *testing.T) {
	engine, err := drel.NewEngine("file:" + filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("the engine does not open: %v", err)
	}
	defer engine.Close()

	err = engine.WithTx(context.Background(), func(ctx context.Context) error {
		repo, err := avero.Repo(ctx, drel.ModelMeta[row]{})
		if err != nil {
			return err
		}
		if repo == nil {
			t.Fatal("Repo returned no repository inside a transaction")
		}
		return avero.Save(ctx)
	})
	if err != nil {
		t.Fatalf("the transaction does not run: %v", err)
	}
}

// Save outside a transaction names the same repair as Repo.
func TestSaveWithoutATransaction(t *testing.T) {
	if err := avero.Save(context.Background()); err == nil {
		t.Fatal("Save returned no error outside a transaction")
	}
}
