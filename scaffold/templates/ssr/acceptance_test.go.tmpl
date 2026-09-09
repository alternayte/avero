package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/avero"
	"github.com/alternayte/drel"
)

// The acceptance suite drives real HTTP against a real database. It uses no
// mock. See AGENTS.md, the test rules.

// app returns the handler of the application and the database behind it.
func app(t *testing.T) http.Handler {
	t.Helper()
	file := filepath.Join(t.TempDir(), "test.db")
	engine, err := drel.NewEngine("file:" + file)
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

// get performs one GET request.
func get(t *testing.T, h http.Handler, path string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// post performs one POST request with a form.
func post(t *testing.T, h http.Handler, path string, form url.Values, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// token returns the CSRF token of a page and the cookies of the answer.
func token(t *testing.T, rec *httptest.ResponseRecorder) (string, []*http.Cookie) {
	t.Helper()
	mark := `name="_csrf" value="`
	body := rec.Body.String()
	i := strings.Index(body, mark)
	if i < 0 {
		t.Fatalf("the page holds no CSRF token:\n%s", body)
	}
	rest := body[i+len(mark):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatalf("the CSRF field does not close:\n%s", body)
	}
	return rest[:end], rec.Result().Cookies()
}

func TestTheListPageAnswers(t *testing.T) {
	rec := get(t, app(t), "/", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "No post exists yet.") {
		t.Fatalf("the page holds %q", rec.Body.String())
	}
}

func TestAValidFormWritesAPostAndShowsTheToast(t *testing.T) {
	h := app(t)
	form := get(t, h, "/posts/new", nil)
	csrf, cookies := token(t, form)

	created := post(t, h, "/posts", url.Values{
		"title": {"A title"}, "body": {"A body"}, "_csrf": {csrf},
	}, cookies)
	if created.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303:\n%s", created.Code, created.Body.String())
	}

	list := get(t, h, "/", created.Result().Cookies())
	body := list.Body.String()
	if !strings.Contains(body, "A title") {
		t.Fatalf("the list holds no post:\n%s", body)
	}
	if !strings.Contains(body, "The post is saved") {
		t.Fatalf("the list holds no toast:\n%s", body)
	}
}

func TestAnInvalidFormRendersTheOldInputAndTheFieldError(t *testing.T) {
	h := app(t)
	form := get(t, h, "/posts/new", nil)
	csrf, cookies := token(t, form)

	rec := post(t, h, "/posts", url.Values{
		"title": {"no"}, "body": {""}, "_csrf": {csrf},
	}, cookies)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `value="no"`) {
		t.Fatalf("the form holds no old input:\n%s", body)
	}
	if !strings.Contains(body, `class="error"`) {
		t.Fatalf("the form holds no field error:\n%s", body)
	}
}

func TestAFormWithNoTokenReturnsTheCSRFStatus(t *testing.T) {
	rec := post(t, app(t), "/posts", url.Values{"title": {"A title"}, "body": {"A body"}}, nil)
	if rec.Code != avero.StatusCSRF {
		t.Fatalf("status = %d, want %d", rec.Code, avero.StatusCSRF)
	}
}

func TestAPostReadsAndDeletes(t *testing.T) {
	h := app(t)
	form := get(t, h, "/posts/new", nil)
	csrf, cookies := token(t, form)
	created := post(t, h, "/posts", url.Values{
		"title": {"A title"}, "body": {"A body"}, "_csrf": {csrf},
	}, cookies)

	list := get(t, h, "/", created.Result().Cookies())
	id := idOf(t, list.Body.String())

	show := get(t, h, "/posts/"+id, nil)
	if show.Code != http.StatusOK || !strings.Contains(show.Body.String(), "A body") {
		t.Fatalf("the post page answered %d:\n%s", show.Code, show.Body.String())
	}

	page := get(t, h, "/posts/"+id, nil)
	deleteToken, deleteCookies := token(t, page)
	removed := post(t, h, "/posts/"+id+"/delete", url.Values{"_csrf": {deleteToken}}, deleteCookies)
	if removed.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", removed.Code)
	}
	if again := get(t, h, "/posts/"+id, nil); again.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 after the delete", again.Code)
	}
}

func TestAnAbsentPostAnswersFourOhFour(t *testing.T) {
	rec := get(t, app(t), "/posts/absent", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// idOf returns the identifier of the first post of a list page.
func idOf(t *testing.T, body string) string {
	t.Helper()
	mark := `<li><a href="/posts/`
	i := strings.Index(body, mark)
	if i < 0 {
		t.Fatalf("the list holds no post:\n%s", body)
	}
	rest := body[i+len(mark):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatalf("the link does not close:\n%s", body)
	}
	return rest[:end]
}

// The environment of the suite must not reach the application.
var _ = os.Getenv
