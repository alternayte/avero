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
	if _, err := engine.ApplyMigrationsFS(context.Background(), migrationSets()...); err != nil {
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

func TestTheApplicationServesTheFrontEnd(t *testing.T) {
	// The binary carries the build of the front end, so a request for a path
	// of the front end answers the index document from the embedded file
	// system. One binary holds the server and the front end.
	h := app(t)
	for _, path := range []string{"/", "/tasks/7"} {
		rec := call(t, h, http.MethodGet, path, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s answered %d, want 200", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `id="root"`) {
			t.Fatalf("%s holds no document of the front end:\n%s", path, rec.Body.String())
		}
	}
	// A path of the API stays with the API.
	if rec := call(t, h, http.MethodGet, "/api/tasks", ""); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), "tasks") {
		t.Fatalf("the API answered %d:\n%s", rec.Code, rec.Body.String())
	}
}
