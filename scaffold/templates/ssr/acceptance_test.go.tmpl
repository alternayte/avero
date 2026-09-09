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
//
// A form carries the token in a hidden field. A page with no form carries it
// in the signal that Datastar reads, so the test reads both.
func token(t *testing.T, rec *httptest.ResponseRecorder) (string, []*http.Cookie) {
	t.Helper()
	body := rec.Body.String()
	for _, mark := range []string{`name="_csrf" value="`, `data-signals-csrf="&#39;`} {
		i := strings.Index(body, mark)
		if i < 0 {
			continue
		}
		rest := body[i+len(mark):]
		end := strings.IndexAny(rest, `"&`)
		if end < 0 {
			t.Fatalf("the CSRF value does not close:\n%s", body)
		}
		return rest[:end], rec.Result().Cookies()
	}
	t.Fatalf("the page holds no CSRF token:\n%s", body)
	return "", nil
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
	// The toaster of Basecoat holds the message. The server renders it,
	// because Basecoat watches the document and initializes a toast that
	// arrives later. See the design of 2026-09-09.
	if !strings.Contains(body, `id="toaster"`) {
		t.Fatalf("the list holds no toaster:\n%s", body)
	}
	if !strings.Contains(body, `data-category="success"`) {
		t.Fatalf("the toast states no category:\n%s", body)
	}
	if !strings.Contains(body, "data-toast-cancel") {
		t.Fatalf("the toast carries no dismiss button:\n%s", body)
	}
}

func TestTheToasterStandsWithNoToast(t *testing.T) {
	rec := get(t, app(t), "/", nil)

	body := rec.Body.String()
	if !strings.Contains(body, `id="toaster"`) {
		t.Fatalf("the page holds no toaster:\n%s", body)
	}
	if strings.Contains(body, `class="toast"`) {
		t.Fatalf("the page holds a toast although no message exists:\n%s", body)
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
	if !strings.Contains(body, `class="text-destructive text-sm"`) {
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

	// Datastar sends the delete with the token in the header, and the answer
	// is a patch that removes the row. See S12.
	page := get(t, h, "/posts/"+id, nil)
	deleteToken, deleteCookies := token(t, page)
	req := httptest.NewRequest(http.MethodDelete, "/posts/"+id, nil)
	req.Header.Set("X-CSRF-Token", deleteToken)
	for _, c := range deleteCookies {
		req.AddCookie(c)
	}
	removed := httptest.NewRecorder()
	h.ServeHTTP(removed, req)

	if removed.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", removed.Code)
	}
	if got := removed.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want a Datastar stream", got)
	}
	body := removed.Body.String()
	if !strings.Contains(body, "event: datastar-patch-elements") ||
		!strings.Contains(body, "data: selector #post-"+id) ||
		!strings.Contains(body, "data: mode remove") {
		t.Fatalf("the answer holds no patch that removes the row:\n%s", body)
	}
	if !strings.Contains(body, "No post exists yet.") {
		t.Fatalf("the answer states no empty list:\n%s", body)
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
	mark := `<li id="post-`
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

func TestTheApplicationServesItsAssets(t *testing.T) {
	// The binary carries the built assets, so a request for a hashed name
	// reads the embedded file system and no directory beside the binary.
	h := app(t)
	page := get(t, h, "/", nil)
	href := hrefOf(t, page.Body.String(), "stylesheet")

	rec := get(t, h, href, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("the asset %s answered %d, want 200", href, rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got == "" {
		t.Fatal("the asset carries no cache header")
	}
	if absent := get(t, h, "/assets/absent.css", nil); absent.Code != http.StatusNotFound {
		t.Fatalf("an absent asset answered %d, want 404", absent.Code)
	}
}

// hrefOf returns the address of the stylesheet of a page.
func hrefOf(t *testing.T, body, rel string) string {
	t.Helper()
	mark := `<link rel="` + rel + `" href="`
	i := strings.Index(body, mark)
	if i < 0 {
		t.Fatalf("the page holds no %s:\n%s", rel, body)
	}
	rest := body[i+len(mark):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatalf("the address does not close:\n%s", body)
	}
	return rest[:end]
}

// The environment of the suite must not reach the application.
var _ = os.Getenv
