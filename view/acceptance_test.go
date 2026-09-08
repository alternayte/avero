package view_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/alternayte/avero/router"
	"github.com/alternayte/avero/view"
)

// postForm is the shape that `avero generate` writes for a form.
type postForm struct {
	Title string
}

func (p *postForm) Bind(c *router.Ctx) error {
	if err := c.Request().ParseForm(); err != nil {
		return err
	}
	p.Title = c.Request().PostFormValue("title")
	return nil
}

func (p *postForm) Validate(_ *router.Ctx, f *router.Fields) {
	if strings.TrimSpace(p.Title) == "" {
		f.Add("title", "Write a title")
	}
}

// create answers a valid form. It sets a toast and redirects.
func create(c *router.Ctx, _ postForm) (router.Response, error) {
	c.Success("The post is saved")
	return router.Redirect(http.StatusSeeOther, "/posts/new"), nil
}

// app builds the router of the acceptance tests.
func app(t *testing.T, mws ...router.Middleware) http.Handler {
	t.Helper()
	r := router.New(view.WithForm(func(*router.Ctx, *router.Fields) view.Component {
		return page("New post")
	}))
	r.Use(mws...)
	r.Get("/posts/new", func(*router.Ctx) (router.Response, error) {
		return view.View(page("New post")), nil
	})
	r.Post("/posts", router.In(create))
	h, err := r.Handler()
	if err != nil {
		t.Fatalf("Handler returned %v, want nil", err)
	}
	return h
}

func TestAnInvalidFormPostReturnsTheOldInputAndTheFieldError(t *testing.T) {
	h := app(t)
	body := url.Values{"title": {"   "}}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/posts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	page := rec.Body.String()
	if !strings.Contains(page, `<input name="title" value="   ">`) {
		t.Fatalf("the form does not hold the old input:\n%s", page)
	}
	if !strings.Contains(page, `<span class="error">Write a title</span>`) {
		t.Fatalf("the form does not hold the field error:\n%s", page)
	}
	if strings.Index(page, `name="title"`) > strings.Index(page, `class="error"`) {
		t.Fatalf("the error stands before the field:\n%s", page)
	}
}

func TestACSRFFaultReturnsFourNineteenAndDoesNotReachTheHandler(t *testing.T) {
	reached := false
	r := router.New()
	r.Use(router.CSRF(secret()))
	r.Post("/posts", func(*router.Ctx) (router.Response, error) {
		reached = true
		return router.NoContent(), nil
	})
	h, err := r.Handler()
	if err != nil {
		t.Fatalf("Handler returned %v, want nil", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/posts", strings.NewReader("title=a"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != router.StatusCSRF {
		t.Fatalf("status = %d, want %d", rec.Code, router.StatusCSRF)
	}
	if reached {
		t.Fatal("the handler ran, want no call")
	}
}

func TestTheCSRFFieldCarriesTheTokenOfTheRequest(t *testing.T) {
	h := app(t, router.CSRF(secret()))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/posts/new", nil))
	token := field(t, rec.Body.String(), router.CSRFFieldName)
	if token == "" {
		t.Fatalf("the page holds no CSRF token:\n%s", rec.Body.String())
	}

	// The same token in the form and in the cookie reaches the handler.
	body := url.Values{"title": {""}, router.CSRFFieldName: {token}}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/posts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	post := httptest.NewRecorder()
	h.ServeHTTP(post, req)
	if post.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", post.Code)
	}
}

func TestAToastSetBeforeARedirectAppearsAfterTheRedirect(t *testing.T) {
	h := app(t, router.Flash(secret()))

	body := url.Values{"title": {"A title"}}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/posts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post := httptest.NewRecorder()
	h.ServeHTTP(post, req)
	if post.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", post.Code)
	}

	next := httptest.NewRequest(http.MethodGet, "/posts/new", nil)
	for _, c := range post.Result().Cookies() {
		next.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, next)
	if !strings.Contains(rec.Body.String(), `<p class="success">The post is saved</p>`) {
		t.Fatalf("the page holds no toast:\n%s", rec.Body.String())
	}

	// The flash is read one time. A second request shows no toast.
	third := httptest.NewRequest(http.MethodGet, "/posts/new", nil)
	for _, c := range rec.Result().Cookies() {
		third.AddCookie(c)
	}
	again := httptest.NewRecorder()
	h.ServeHTTP(again, third)
	if strings.Contains(again.Body.String(), "The post is saved") {
		t.Fatalf("the toast appeared two times:\n%s", again.Body.String())
	}
}

// field returns the value of a hidden input.
func field(t *testing.T, page, name string) string {
	t.Helper()
	mark := `name="` + name + `" value="`
	i := strings.Index(page, mark)
	if i < 0 {
		return ""
	}
	rest := page[i+len(mark):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}
