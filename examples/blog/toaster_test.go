package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The server renders the toaster of Basecoat and each toast that it holds.
// Basecoat watches the document and starts a toast that arrives later, so the
// dismiss button and the timer of a toast need no script of the application.

// TestAValidFormShowsTheBasecoatToast proves that a saved post shows a toast
// with the markup that Basecoat states.
func TestAValidFormShowsTheBasecoatToast(t *testing.T) {
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

// TestTheToasterStandsWithNoToast proves that the list page carries the
// toaster container even when the page holds no message.
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
