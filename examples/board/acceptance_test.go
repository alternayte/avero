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

func TestTheAPIWritesReadsAndCompletesATask(t *testing.T) {
	h := app(t)
	created := call(t, h, http.MethodPost, "/api/tasks", `{"title":"A title"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201:\n%s", created.Code, created.Body.String())
	}
	var task struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &task); err != nil {
		t.Fatalf("the answer does not parse: %v", err)
	}

	list := call(t, h, http.MethodGet, "/api/tasks", "")
	if !strings.Contains(list.Body.String(), "A title") {
		t.Fatalf("the list holds %q", list.Body.String())
	}

	done := call(t, h, http.MethodPatch, "/api/tasks/"+task.ID, `{"done":true}`)
	if done.Code != http.StatusOK || !strings.Contains(done.Body.String(), `"done":true`) {
		t.Fatalf("the answer is %d %s", done.Code, done.Body.String())
	}

	removed := call(t, h, http.MethodDelete, "/api/tasks/"+task.ID, "")
	if removed.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", removed.Code)
	}
	if again := call(t, h, http.MethodPatch, "/api/tasks/"+task.ID, `{"done":true}`); again.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 after the delete", again.Code)
	}
}

func TestAnInvalidBodyAnswersTheFieldErrors(t *testing.T) {
	rec := call(t, app(t), http.MethodPost, "/api/tasks", `{"title":""}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	var answer struct {
		Errors map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
		t.Fatalf("the answer does not parse: %v", err)
	}
	if answer.Errors["title"] == "" {
		t.Fatalf("the answer holds %v", answer.Errors)
	}
}

func TestTheApplicationServesItsAssets(t *testing.T) {
	// The binary carries the bundle of the front end, so a request for a
	// hashed name reads the embedded file system and no directory beside the
	// binary. One binary holds the server and the front end.
	h := app(t)
	shell := call(t, h, http.MethodGet, "/", "")
	src := srcOf(t, shell.Body.String())

	rec := call(t, h, http.MethodGet, src, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("the bundle %s answered %d, want 200", src, rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("the bundle holds no byte")
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/javascript") &&
		!strings.HasPrefix(got, "application/javascript") {
		t.Fatalf("Content-Type = %q, want a script", got)
	}
	if absent := call(t, h, http.MethodGet, "/assets/absent.js", ""); absent.Code != http.StatusNotFound {
		t.Fatalf("an absent asset answered %d, want 404", absent.Code)
	}
}

// srcOf returns the address of the script of the shell.
func srcOf(t *testing.T, body string) string {
	t.Helper()
	mark := `<script type="module" src="`
	i := strings.Index(body, mark)
	if i < 0 {
		t.Fatalf("the shell holds no script:\n%s", body)
	}
	rest := body[i+len(mark):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatalf("the address does not close:\n%s", body)
	}
	return rest[:end]
}
