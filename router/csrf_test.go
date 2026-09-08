package router_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/alternayte/avero/router"
)

const secret = "a-test-secret-that-is-long-enough"

// tokenFrom performs a GET and returns the CSRF token and its cookie.
func tokenFrom(t *testing.T, h http.Handler) (string, *http.Cookie) {
	t.Helper()
	rec := record(h, newRequest(http.MethodGet, "/form", nil))
	if rec.Code != 200 {
		t.Fatalf("GET /form gave %d", rec.Code)
	}
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	for _, c := range res.Cookies() {
		if c.Name == router.CSRFCookieName {
			return rec.Body.String(), c
		}
	}
	t.Fatal("the response set no CSRF cookie")
	return "", nil
}

// csrfRouter builds a router with the CSRF middleware and two routes.
func csrfRouter(t *testing.T) http.Handler {
	t.Helper()
	r := router.New()
	r.Use(router.CSRF(secret))
	r.Get("/form", func(c *router.Ctx) (router.Response, error) {
		return router.Text(200, c.CSRFToken()), nil
	})
	r.Post("/submit", ok("accepted"))
	h, err := r.Handler()
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}
	return h
}

func TestCSRFSetsATokenOnASafeMethod(t *testing.T) {
	h := csrfRouter(t)
	token, cookie := tokenFrom(t, h)
	if token == "" {
		t.Fatal("the handler read an empty token")
	}
	if cookie.Value == "" {
		t.Fatal("the cookie carries no value")
	}
	if !cookie.HttpOnly {
		t.Fatal("the CSRF cookie is not HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("the cookie SameSite is %v, want Lax", cookie.SameSite)
	}
}

func TestCSRFKeepsOneTokenAcrossRequests(t *testing.T) {
	h := csrfRouter(t)
	first, cookie := tokenFrom(t, h)

	req := newRequest(http.MethodGet, "/form", nil)
	req.AddCookie(cookie)
	second := record(h, req).Body.String()
	if first != second {
		t.Fatalf("the token changed from %q to %q", first, second)
	}
}

func TestCSRFAcceptsAMatchingFormField(t *testing.T) {
	h := csrfRouter(t)
	token, cookie := tokenFrom(t, h)

	form := url.Values{router.CSRFFieldName: {token}}
	req := newRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)

	if rec := record(h, req); rec.Code != 200 || rec.Body.String() != "accepted" {
		t.Fatalf("gave %d %q, want 200 \"accepted\"", rec.Code, rec.Body.String())
	}
}

func TestCSRFAcceptsAMatchingHeader(t *testing.T) {
	h := csrfRouter(t)
	token, cookie := tokenFrom(t, h)

	req := newRequest(http.MethodPost, "/submit", nil)
	req.Header.Set(router.CSRFHeaderName, token)
	req.AddCookie(cookie)

	if rec := record(h, req); rec.Code != 200 {
		t.Fatalf("gave %d, want 200", rec.Code)
	}
}

func TestCSRFRejectsAMissingToken(t *testing.T) {
	h := csrfRouter(t)
	_, cookie := tokenFrom(t, h)

	req := newRequest(http.MethodPost, "/submit", nil)
	req.AddCookie(cookie)
	if rec := record(h, req); rec.Code != router.StatusCSRF {
		t.Fatalf("gave %d, want %d", rec.Code, router.StatusCSRF)
	}
}

func TestCSRFRejectsAWrongToken(t *testing.T) {
	h := csrfRouter(t)
	_, cookie := tokenFrom(t, h)

	req := newRequest(http.MethodPost, "/submit", nil)
	req.Header.Set(router.CSRFHeaderName, "not-the-token")
	req.AddCookie(cookie)
	if rec := record(h, req); rec.Code != router.StatusCSRF {
		t.Fatalf("gave %d, want %d", rec.Code, router.StatusCSRF)
	}
}

func TestCSRFRejectsAMissingCookie(t *testing.T) {
	h := csrfRouter(t)
	token, _ := tokenFrom(t, h)

	req := newRequest(http.MethodPost, "/submit", nil)
	req.Header.Set(router.CSRFHeaderName, token)
	if rec := record(h, req); rec.Code != router.StatusCSRF {
		t.Fatalf("gave %d, want %d", rec.Code, router.StatusCSRF)
	}
}

func TestCSRFRejectsACookieThatAnotherSecretSigned(t *testing.T) {
	h := csrfRouter(t)
	token, cookie := tokenFrom(t, h)

	other := router.New()
	other.Use(router.CSRF("a-different-secret-of-good-length"))
	other.Post("/submit", ok("accepted"))
	oh, err := other.Handler()
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}

	req := newRequest(http.MethodPost, "/submit", nil)
	req.Header.Set(router.CSRFHeaderName, token)
	req.AddCookie(cookie)
	if rec := record(oh, req); rec.Code != router.StatusCSRF {
		t.Fatalf("gave %d, want %d", rec.Code, router.StatusCSRF)
	}
}

func TestCSRFDoesNotReachTheHandlerOnAFault(t *testing.T) {
	reached := false
	r := router.New()
	r.Use(router.CSRF(secret))
	r.Post("/submit", func(*router.Ctx) (router.Response, error) {
		reached = true
		return router.Text(200, "accepted"), nil
	})
	h, err := r.Handler()
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}
	record(h, newRequest(http.MethodPost, "/submit", nil))
	if reached {
		t.Fatal("the handler ran although the CSRF check failed")
	}
}

func TestCSRFLetsASafeMethodThrough(t *testing.T) {
	h := csrfRouter(t)
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		if rec := record(h, newRequest(method, "/form", nil)); rec.Code == router.StatusCSRF {
			t.Fatalf("%s /form was rejected", method)
		}
	}
}

func TestAnEmptySecretIsAFault(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("CSRF accepted an empty secret")
		}
	}()
	router.CSRF("")
}

func TestAShortSecretIsAFault(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("CSRF accepted a short secret")
		}
	}()
	router.CSRF("short")
}
