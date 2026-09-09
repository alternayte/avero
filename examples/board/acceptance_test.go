package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/avero"
	"github.com/alternayte/drel"
)

// The acceptance suite drives real HTTP against a real database. It uses no
// mock. See AGENTS.md, the test rules.

// app returns the handler of the application.
func app(t *testing.T) http.Handler {
	t.Helper()
	engine, err := drel.NewEngine("file:" + filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("the database does not open: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	if _, err := engine.ApplyMigrations(context.Background(), migrationsDir); err != nil {
		t.Fatalf("the migrations do not apply: %v", err)
	}
	cfg := Config{}
	cfg.Secret = avero.Secret(strings.Repeat("k", 64))
	r, _, err := wire(engine, cfg)
	if err != nil {
		t.Fatalf("the wiring failed: %v", err)
	}
	handler, err := r.Handler()
	if err != nil {
		t.Fatalf("the router holds a fault: %v", err)
	}
	return handler
}

// call performs one request and returns the recorder.
func call(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestTheShellAnswersTheRootPath(t *testing.T) {
	rec := call(t, app(t), http.MethodGet, "/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `<div id="app">`) {
		t.Fatalf("the shell holds %q", rec.Body.String())
	}
}

func TestTheAPIWritesAndReadsAPost(t *testing.T) {
	h := app(t)
	created := call(t, h, http.MethodPost, "/api/posts", `{"title":"A title","body":"A body"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201:\n%s", created.Code, created.Body.String())
	}
	var post struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &post); err != nil {
		t.Fatalf("the answer does not parse: %v", err)
	}

	list := call(t, h, http.MethodGet, "/api/posts", "")
	if !strings.Contains(list.Body.String(), "A title") {
		t.Fatalf("the list holds %q", list.Body.String())
	}

	one := call(t, h, http.MethodGet, "/api/posts/"+post.ID, "")
	if one.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", one.Code)
	}

	removed := call(t, h, http.MethodDelete, "/api/posts/"+post.ID, "")
	if removed.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", removed.Code)
	}
	if again := call(t, h, http.MethodGet, "/api/posts/"+post.ID, ""); again.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 after the delete", again.Code)
	}
}

func TestAnInvalidBodyAnswersTheFieldErrors(t *testing.T) {
	rec := call(t, app(t), http.MethodPost, "/api/posts", `{"title":"no","body":""}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	var answer struct {
		Errors map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
		t.Fatalf("the answer does not parse: %v", err)
	}
	if answer.Errors["title"] == "" || answer.Errors["body"] == "" {
		t.Fatalf("the answer holds %v", answer.Errors)
	}
}
