package router_test

import (
	"net/http"
	"testing"

	"github.com/alternayte/avero/router"
)

// flashRouter builds a router that adds a toast and then redirects, plus a
// route that reads the toasts of the next request.
func flashRouter(t *testing.T) http.Handler {
	t.Helper()
	r := router.New()
	r.Use(router.Flash(secret))
	r.Post("/save", func(c *router.Ctx) (router.Response, error) {
		c.Success("saved")
		return router.Redirect(303, "/things"), nil
	})
	r.Get("/things", func(c *router.Ctx) (router.Response, error) {
		if len(c.Toasts()) == 0 {
			return router.Text(200, "none"), nil
		}
		return router.Text(200, c.Toasts()[0].Level+":"+c.Toasts()[0].Message), nil
	})
	r.Get("/plain", func(c *router.Ctx) (router.Response, error) {
		c.Info("shown now")
		return router.Text(200, "page"), nil
	})
	h, err := r.Handler()
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}
	return h
}

// flashCookie returns the flash cookie of a recorder, or nil.
func flashCookie(t *testing.T, rec *http.Response) *http.Cookie {
	t.Helper()
	for _, c := range rec.Cookies() {
		if c.Name == router.FlashCookieName {
			return c
		}
	}
	return nil
}

func TestAToastCrossesARedirect(t *testing.T) {
	h := flashRouter(t)

	rec := record(h, newRequest(http.MethodPost, "/save", nil))
	if rec.Code != 303 {
		t.Fatalf("POST /save gave %d, want 303", rec.Code)
	}
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	cookie := flashCookie(t, res)
	if cookie == nil || cookie.Value == "" {
		t.Fatal("the redirect set no flash cookie")
	}

	next := newRequest(http.MethodGet, "/things", nil)
	next.AddCookie(cookie)
	if body := record(h, next).Body.String(); body != "success:saved" {
		t.Fatalf("the next request read %q, want success:saved", body)
	}
}

func TestAFlashIsReadOneTime(t *testing.T) {
	h := flashRouter(t)
	rec := record(h, newRequest(http.MethodPost, "/save", nil))
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	cookie := flashCookie(t, res)

	first := newRequest(http.MethodGet, "/things", nil)
	first.AddCookie(cookie)
	firstRec := record(h, first)
	if firstRec.Body.String() != "success:saved" {
		t.Fatalf("the first request read %q", firstRec.Body.String())
	}
	// The response clears the cookie.
	firstRes := firstRec.Result()
	defer func() { _ = firstRes.Body.Close() }()
	cleared := flashCookie(t, firstRes)
	if cleared == nil || cleared.MaxAge >= 0 {
		t.Fatalf("the response did not clear the flash cookie: %+v", cleared)
	}
}

func TestAToastOnAPageSetsNoCookie(t *testing.T) {
	h := flashRouter(t)
	rec := record(h, newRequest(http.MethodGet, "/plain", nil))
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	if c := flashCookie(t, res); c != nil && c.Value != "" {
		t.Fatal("a page response set a flash cookie although it shows the toast itself")
	}
}

func TestNoToastSetsNoCookie(t *testing.T) {
	h := flashRouter(t)
	rec := record(h, newRequest(http.MethodGet, "/things", nil))
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	if c := flashCookie(t, res); c != nil && c.Value != "" {
		t.Fatal("a response with no toast set a flash cookie")
	}
}

func TestAFlashCookieThatAnotherSecretSignedIsIgnored(t *testing.T) {
	h := flashRouter(t)
	rec := record(h, newRequest(http.MethodPost, "/save", nil))
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	cookie := flashCookie(t, res)

	other := router.New()
	other.Use(router.Flash("a-different-secret-of-good-length"))
	other.Get("/things", func(c *router.Ctx) (router.Response, error) {
		if len(c.Toasts()) == 0 {
			return router.Text(200, "none"), nil
		}
		return router.Text(200, "read"), nil
	})
	oh, err := other.Handler()
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}
	req := newRequest(http.MethodGet, "/things", nil)
	req.AddCookie(cookie)
	if body := record(oh, req).Body.String(); body != "none" {
		t.Fatalf("a cookie with a wrong signature was read: %q", body)
	}
}
