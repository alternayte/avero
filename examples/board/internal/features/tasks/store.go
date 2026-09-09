package tasks

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/alternayte/drel"
)

// Row is one row of the tasks table. The API answers a Task, which
// handlers.go states, so the shape of the table and the shape of the answer
// change apart.
type Row struct {
	// ID identifies the task.
	ID string
	// Title is the name that a person reads.
	Title string
	// Done states whether the task is complete.
	Done bool
	// CreatedAt is the time of the write.
	CreatedAt time.Time
}

// Store reads and writes the tasks. It takes the engine as a field, so a test
// passes its own database. See design rule 3.
type Store struct {
	engine *drel.Engine
}

// NewStore builds the store.
func NewStore(engine *drel.Engine) *Store { return &Store{engine: engine} }

// List returns the tasks, newest first. An empty search returns every task.
func (s *Store) List(ctx context.Context, search string) ([]Row, error) {
	sql := "SELECT id, title, done FROM tasks ORDER BY created_at DESC"
	args := []any{}
	if search != "" {
		sql = "SELECT id, title, done FROM tasks WHERE title LIKE " + s.mark(1) +
			" ORDER BY created_at DESC"
		args = append(args, "%"+search+"%")
	}
	rows, err := s.query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("the tasks do not read: %w", err)
	}
	defer rows.Close()

	var out []Row
	for rows.Next() {
		var row Row
		if err := rows.Scan(&row.ID, &row.Title, &row.Done); err != nil {
			return nil, fmt.Errorf("a task does not read: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// Get returns one task and reports whether the table holds it.
func (s *Store) Get(ctx context.Context, id string) (Row, bool, error) {
	rows, err := s.query(ctx, "SELECT id, title, done FROM tasks WHERE id = "+s.mark(1), id)
	if err != nil {
		return Row{}, false, fmt.Errorf("the task does not read: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return Row{}, false, rows.Err()
	}
	var row Row
	if err := rows.Scan(&row.ID, &row.Title, &row.Done); err != nil {
		return Row{}, false, fmt.Errorf("the task does not read: %w", err)
	}
	return row, true, nil
}

// SetDone marks one task complete, or open again. It returns the task and
// reports whether the table holds it.
func (s *Store) SetDone(ctx context.Context, id string, done bool) (Row, bool, error) {
	sql := fmt.Sprintf("UPDATE tasks SET done = %s WHERE id = %s", s.mark(1), s.mark(2))
	if err := s.exec(ctx, sql, done, id); err != nil {
		return Row{}, false, fmt.Errorf("the task does not write: %w", err)
	}
	return s.Get(ctx, id)
}

// Create writes one task and returns its identifier.
func (s *Store) Create(ctx context.Context, title string) (string, error) {
	id := newID()
	sql := fmt.Sprintf("INSERT INTO tasks (id, title, done, created_at) VALUES (%s, %s, %s, %s)",
		s.mark(1), s.mark(2), s.mark(3), s.mark(4))
	if err := s.exec(ctx, sql, id, title, false, time.Now().UTC()); err != nil {
		return "", fmt.Errorf("the task does not write: %w", err)
	}
	return id, nil
}

// Delete removes one task.
func (s *Store) Delete(ctx context.Context, id string) error {
	if err := s.exec(ctx, "DELETE FROM tasks WHERE id = "+s.mark(1), id); err != nil {
		return fmt.Errorf("the task does not delete: %w", err)
	}
	return nil
}

// query runs a read inside the transaction of the request when one is open.
func (s *Store) query(ctx context.Context, sql string, args ...any) (drel.Rows, error) {
	if tx, ok := drel.FromContext(ctx); ok {
		return tx.Query(ctx, sql, args...)
	}
	return s.engine.Query(ctx, sql, args...)
}

// exec runs a write inside the transaction of the request when one is open, so
// the write and the response commit together. See S4.
func (s *Store) exec(ctx context.Context, sql string, args ...any) error {
	if tx, ok := drel.FromContext(ctx); ok {
		_, err := tx.Exec(ctx, sql, args...)
		return err
	}
	_, err := s.engine.Exec(ctx, sql, args...)
	return err
}

// mark returns the parameter mark of the dialect. PostgreSQL counts its
// parameters and SQLite does not.
func (s *Store) mark(n int) string {
	if s.engine != nil && s.engine.DialectName() == "taskgres" {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// newID returns a random identifier.
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b)
}
