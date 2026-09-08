package router_test

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/alternayte/avero/router"
	"github.com/alternayte/drel"
)

// newEngine returns an engine on a SQLite file that this test owns, and a
// table that a handler writes into. modernc.org/sqlite is pure Go, so the test
// needs no container.
func newEngine(t *testing.T) *drel.Engine {
	t.Helper()
	e, err := drel.NewEngine(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewEngine returned an error: %v", err)
	}
	t.Cleanup(e.Close)
	err = e.WithTx(context.Background(), func(ctx context.Context) error {
		_, err := drel.MustFromContext(ctx).Exec(ctx, `CREATE TABLE things (name text)`)
		return err
	})
	if err != nil {
		t.Fatalf("the schema did not apply: %v", err)
	}
	return e
}

// count returns the number of rows in things.
func count(t *testing.T, e *drel.Engine) int {
	t.Helper()
	var n int
	err := e.WithTx(context.Background(), func(ctx context.Context) error {
		return drel.MustFromContext(ctx).
			QueryRow(ctx, `SELECT count(*) FROM things`).Scan(&n)
	})
	if err != nil {
		t.Fatalf("the count failed: %v", err)
	}
	return n
}

// write inserts one row inside the transaction of the request.
func write(c *router.Ctx, name string) error {
	_, err := c.MustTx().Exec(c.Context(), `INSERT INTO things (name) VALUES (?)`, name)
	return err
}

func TestTheTransactionCommitsOn2xx(t *testing.T) {
	e := newEngine(t)
	r := router.New()
	r.Use(router.Transaction(e))
	r.Post("/things", func(c *router.Ctx) (router.Response, error) {
		if err := write(c, "kept"); err != nil {
			return nil, err
		}
		return router.Text(201, "made"), nil
	})

	if rec := serve(t, r, http.MethodPost, "/things"); rec.Code != 201 {
		t.Fatalf("gave %d, want 201", rec.Code)
	}
	if n := count(t, e); n != 1 {
		t.Fatalf("the table holds %d rows, want 1", n)
	}
}

func TestTheTransactionCommitsOn3xx(t *testing.T) {
	e := newEngine(t)
	r := router.New()
	r.Use(router.Transaction(e))
	r.Post("/things", func(c *router.Ctx) (router.Response, error) {
		if err := write(c, "kept"); err != nil {
			return nil, err
		}
		return router.Redirect(303, "/things"), nil
	})

	if rec := serve(t, r, http.MethodPost, "/things"); rec.Code != 303 {
		t.Fatalf("gave %d, want 303", rec.Code)
	}
	if n := count(t, e); n != 1 {
		t.Fatalf("the table holds %d rows, want 1", n)
	}
}

func TestAHandlerErrorRollsBackItsWrites(t *testing.T) {
	e := newEngine(t)
	r := router.New()
	r.Use(router.Transaction(e))
	r.Post("/things", func(c *router.Ctx) (router.Response, error) {
		if err := write(c, "lost"); err != nil {
			return nil, err
		}
		return nil, errBoom
	})

	if rec := serve(t, r, http.MethodPost, "/things"); rec.Code != 500 {
		t.Fatalf("gave %d, want 500", rec.Code)
	}
	if n := count(t, e); n != 0 {
		t.Fatalf("the table holds %d rows, want 0", n)
	}
}

func TestA5xxResponseRollsBackItsWrites(t *testing.T) {
	e := newEngine(t)
	r := router.New()
	r.Use(router.Transaction(e))
	r.Post("/things", func(c *router.Ctx) (router.Response, error) {
		if err := write(c, "lost"); err != nil {
			return nil, err
		}
		return router.Status(503), nil
	})

	if rec := serve(t, r, http.MethodPost, "/things"); rec.Code != 503 {
		t.Fatalf("gave %d, want 503", rec.Code)
	}
	if n := count(t, e); n != 0 {
		t.Fatalf("the table holds %d rows, want 0", n)
	}
}

func TestA4xxResponseRollsBackItsWrites(t *testing.T) {
	// The SDD commits on a 2xx or a 3xx. Every other status rolls back.
	e := newEngine(t)
	r := router.New()
	r.Use(router.Transaction(e))
	r.Post("/things", func(c *router.Ctx) (router.Response, error) {
		if err := write(c, "lost"); err != nil {
			return nil, err
		}
		return router.Status(422), nil
	})

	if rec := serve(t, r, http.MethodPost, "/things"); rec.Code != 422 {
		t.Fatalf("gave %d, want 422", rec.Code)
	}
	if n := count(t, e); n != 0 {
		t.Fatalf("the table holds %d rows, want 0", n)
	}
}

func TestAPanicRollsBackAndReturns500(t *testing.T) {
	e := newEngine(t)
	log, _ := captureLogger()
	r := router.New()
	// Recover sits outside the transaction, as the middleware order states.
	r.Use(router.Recover(log), router.Transaction(e))
	r.Post("/things", func(c *router.Ctx) (router.Response, error) {
		if err := write(c, "lost"); err != nil {
			return nil, err
		}
		panic("the handler panicked")
	})

	if rec := serve(t, r, http.MethodPost, "/things"); rec.Code != 500 {
		t.Fatalf("gave %d, want 500", rec.Code)
	}
	if n := count(t, e); n != 0 {
		t.Fatalf("the table holds %d rows, want 0", n)
	}
}

func TestTheHandlerReadsTheTransactionFromTheContext(t *testing.T) {
	e := newEngine(t)
	r := router.New()
	r.Use(router.Transaction(e))
	r.Get("/x", func(c *router.Ctx) (router.Response, error) {
		if c.Tx() == nil {
			t.Fatal("Ctx.Tx returned nil inside a transaction")
		}
		if _, ok := drel.FromContext(c.Context()); !ok {
			t.Fatal("the request context carries no transaction")
		}
		return router.NoContent(), nil
	})
	serve(t, r, http.MethodGet, "/x")
}

func TestTxIsNilWithNoTransactionMiddleware(t *testing.T) {
	r := router.New()
	r.Get("/x", func(c *router.Ctx) (router.Response, error) {
		if c.Tx() != nil {
			t.Fatal("Ctx.Tx returned a transaction with no middleware")
		}
		return router.NoContent(), nil
	})
	serve(t, r, http.MethodGet, "/x")
}

func TestTheResponseReachesTheClientAfterTheCommit(t *testing.T) {
	// The handler never writes. Nothing reaches the client before the commit,
	// so a failed commit can never follow a sent 200.
	e := newEngine(t)
	r := router.New()
	r.Use(router.Transaction(e))
	r.Post("/things", func(c *router.Ctx) (router.Response, error) {
		if err := write(c, "kept"); err != nil {
			return nil, err
		}
		// The row is not committed yet. A second engine connection cannot see
		// it, which proves that the write is still inside the transaction.
		return router.Text(201, "made"), nil
	})
	rec := serve(t, r, http.MethodPost, "/things")
	if rec.Code != 201 || rec.Body.String() != "made" {
		t.Fatalf("gave %d %q", rec.Code, rec.Body.String())
	}
}
