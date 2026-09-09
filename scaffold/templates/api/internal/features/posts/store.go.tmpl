package posts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/alternayte/drel"
)

// Post is one row of the posts table.
type Post struct {
	// ID identifies the post.
	ID string
	// Title is the name that a person reads.
	Title string
	// Body is the text of the post.
	Body string
	// CreatedAt is the time of the write.
	CreatedAt time.Time
}

// Store reads and writes the posts. It takes the engine as a field, so a test
// passes its own database. See design rule 3.
type Store struct {
	engine *drel.Engine
}

// NewStore builds the store.
func NewStore(engine *drel.Engine) *Store { return &Store{engine: engine} }

// List returns the posts, newest first. An empty search returns every post.
func (s *Store) List(ctx context.Context, search string) ([]Post, error) {
	sql := "SELECT id, title, body FROM posts ORDER BY created_at DESC"
	args := []any{}
	if search != "" {
		sql = "SELECT id, title, body FROM posts WHERE title LIKE " + s.mark(1) +
			" ORDER BY created_at DESC"
		args = append(args, "%"+search+"%")
	}
	rows, err := s.query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("the posts do not read: %w", err)
	}
	defer rows.Close()

	var out []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Body); err != nil {
			return nil, fmt.Errorf("a post does not read: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Get returns one post and reports whether the table holds it.
func (s *Store) Get(ctx context.Context, id string) (Post, bool, error) {
	rows, err := s.query(ctx, "SELECT id, title, body FROM posts WHERE id = "+s.mark(1), id)
	if err != nil {
		return Post{}, false, fmt.Errorf("the post does not read: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return Post{}, false, rows.Err()
	}
	var p Post
	if err := rows.Scan(&p.ID, &p.Title, &p.Body); err != nil {
		return Post{}, false, fmt.Errorf("the post does not read: %w", err)
	}
	return p, true, nil
}

// Create writes one post and returns its identifier.
func (s *Store) Create(ctx context.Context, title, body string) (string, error) {
	id := newID()
	sql := fmt.Sprintf("INSERT INTO posts (id, title, body, created_at) VALUES (%s, %s, %s, %s)",
		s.mark(1), s.mark(2), s.mark(3), s.mark(4))
	if err := s.exec(ctx, sql, id, title, body, time.Now().UTC()); err != nil {
		return "", fmt.Errorf("the post does not write: %w", err)
	}
	return id, nil
}

// Delete removes one post.
func (s *Store) Delete(ctx context.Context, id string) error {
	if err := s.exec(ctx, "DELETE FROM posts WHERE id = "+s.mark(1), id); err != nil {
		return fmt.Errorf("the post does not delete: %w", err)
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
	if s.engine != nil && s.engine.DialectName() == "postgres" {
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
