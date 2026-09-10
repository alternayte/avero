package tasks

import (
	"context"
	"fmt"

	"github.com/alternayte/avero"
	"github.com/alternayte/drel"
	"github.com/google/uuid"

	"board/internal/features/tasks/model"
)

// Store reads and writes the tasks. It takes the engine as a field, so a test
// passes its own database. See design rule 3.
//
// Every call runs inside the transaction of the request, which the transaction
// middleware opens. avero.Repo returns the repository of that transaction, and
// avero.Save flushes the staged changes. See S4.
type Store struct {
	engine *drel.Engine
}

// NewStore builds the store.
func NewStore(engine *drel.Engine) *Store { return &Store{engine: engine} }

// List returns the tasks, newest first. An empty search returns every task.
func (s *Store) List(ctx context.Context, search string) ([]*model.Task, error) {
	repo, err := avero.Repo(ctx, model.TaskMeta)
	if err != nil {
		return nil, err
	}
	query := repo.AsNoTracking().OrderBy(model.Tasks.ID.Desc())
	if search != "" {
		query = query.Where(model.Tasks.Title.Contains(search))
	}
	rows, err := query.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("the tasks do not read: %w", err)
	}
	return rows, nil
}

// Get returns one task and reports whether the table holds it. An identifier
// that is no UUID holds no row.
func (s *Store) Get(ctx context.Context, id string) (*model.Task, bool, error) {
	key, err := uuid.Parse(id)
	if err != nil {
		return nil, false, nil
	}
	repo, err := avero.Repo(ctx, model.TaskMeta)
	if err != nil {
		return nil, false, err
	}
	task, err := repo.AsNoTracking().Where(model.Tasks.ID.Eq(key)).FirstOrNil(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("the task does not read: %w", err)
	}
	return task, task != nil, nil
}

// SetDone marks one task complete, or open again.
//
// The repository tracks the task, so the change of the field is the whole
// write. drel sends only the column that changed at the commit.
func (s *Store) SetDone(ctx context.Context, id string, done bool) (*model.Task, bool, error) {
	key, err := uuid.Parse(id)
	if err != nil {
		return nil, false, nil
	}
	repo, err := avero.Repo(ctx, model.TaskMeta)
	if err != nil {
		return nil, false, err
	}
	task, err := repo.Where(model.Tasks.ID.Eq(key)).FirstOrNil(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("the task does not read: %w", err)
	}
	if task == nil {
		return nil, false, nil
	}
	task.Done = done
	if err := avero.Save(ctx); err != nil {
		return nil, false, err
	}
	return task, true, nil
}

// Create writes one task. The identifier stands at once, because drel stamps
// the UUID at Add.
func (s *Store) Create(ctx context.Context, title string) (*model.Task, error) {
	repo, err := avero.Repo(ctx, model.TaskMeta)
	if err != nil {
		return nil, err
	}
	task := model.NewTask(title)
	repo.Add(task)
	if err := avero.Save(ctx); err != nil {
		return nil, err
	}
	return task, nil
}

// Delete removes one task. It removes nothing when the table holds no such
// row.
func (s *Store) Delete(ctx context.Context, id string) error {
	key, err := uuid.Parse(id)
	if err != nil {
		return nil
	}
	repo, err := avero.Repo(ctx, model.TaskMeta)
	if err != nil {
		return err
	}
	task, err := repo.Where(model.Tasks.ID.Eq(key)).FirstOrNil(ctx)
	if err != nil {
		return fmt.Errorf("the task does not read: %w", err)
	}
	if task == nil {
		return nil
	}
	if err := repo.Remove(task); err != nil {
		return fmt.Errorf("the task does not delete: %w", err)
	}
	return avero.Save(ctx)
}
