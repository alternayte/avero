package posts

import (
	"context"
	"errors"
	"fmt"

	"github.com/alternayte/drel"
	"github.com/google/uuid"

	"orders/internal/features/posts/model"
)

// Store reads and writes the posts. It takes the engine as a field, so a test
// passes its own database. See design rule 3.
//
// Every call runs inside the transaction of the request, which the transaction
// middleware opens. A read therefore reads its own writes, and a write stages
// the change on the transaction. drel flushes it at the commit, so only the
// columns that changed reach the database. See S4.
type Store struct {
	engine *drel.Engine
}

// NewStore builds the store.
func NewStore(engine *drel.Engine) *Store { return &Store{engine: engine} }

// List returns the posts, newest first. An empty search returns every post.
func (s *Store) List(ctx context.Context, search string) ([]*model.Post, error) {
	repo, err := s.repo(ctx)
	if err != nil {
		return nil, err
	}
	query := repo.AsNoTracking().OrderBy(model.Posts.ID.Desc())
	if search != "" {
		query = query.Where(model.Posts.Title.Contains(search))
	}
	rows, err := query.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("the posts do not read: %w", err)
	}
	return rows, nil
}

// Get returns one post and reports whether the table holds it. An identifier
// that is no UUID holds no row.
func (s *Store) Get(ctx context.Context, id string) (*model.Post, bool, error) {
	key, err := uuid.Parse(id)
	if err != nil {
		return nil, false, nil
	}
	repo, err := s.repo(ctx)
	if err != nil {
		return nil, false, err
	}
	post, err := repo.AsNoTracking().Where(model.Posts.ID.Eq(key)).FirstOrNil(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("the post does not read: %w", err)
	}
	return post, post != nil, nil
}

// Create writes one post. The identifier stands at once, because drel stamps
// the UUID at Add.
func (s *Store) Create(ctx context.Context, title, body string) (*model.Post, error) {
	repo, err := s.repo(ctx)
	if err != nil {
		return nil, err
	}
	post := model.NewPost(title, body)
	repo.Add(post)
	if err := s.save(ctx); err != nil {
		return nil, err
	}
	return post, nil
}

// Delete removes one post. It removes nothing when the table holds no such
// row.
func (s *Store) Delete(ctx context.Context, id string) error {
	key, err := uuid.Parse(id)
	if err != nil {
		return nil
	}
	repo, err := s.repo(ctx)
	if err != nil {
		return err
	}
	post, err := repo.Where(model.Posts.ID.Eq(key)).FirstOrNil(ctx)
	if err != nil {
		return fmt.Errorf("the post does not read: %w", err)
	}
	if post == nil {
		return nil
	}
	if err := repo.Remove(post); err != nil {
		return fmt.Errorf("the post does not delete: %w", err)
	}
	return s.save(ctx)
}

// repo returns the repository of the transaction of the request.
//
// The transaction middleware opens it, so a handler always holds one. A job
// that reads outside a request opens its own with engine.WithTx.
func (s *Store) repo(ctx context.Context) (*drel.TxRepository[model.Post], error) {
	tx, ok := drel.FromContext(ctx)
	if !ok {
		return nil, errors.New("the call needs a transaction: register avero.Transaction in wire.go, or wrap the call in engine.WithTx")
	}
	return drel.NewTxRepository(tx, model.PostMeta), nil
}

// save writes the staged changes to the database inside the transaction of the
// request.
//
// The transaction still commits at the end of the request. The flush only
// makes a later read of the same request see the write.
func (s *Store) save(ctx context.Context) error {
	tx, ok := drel.FromContext(ctx)
	if !ok {
		return errors.New("the call needs a transaction: register avero.Transaction in wire.go, or wrap the call in engine.WithTx")
	}
	if err := tx.SaveChanges(ctx); err != nil {
		return fmt.Errorf("the change does not write: %w", err)
	}
	return nil
}
