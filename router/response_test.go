package router_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alternayte/avero/router"
)

var errBoom = errors.New("the handler failed")

// respond runs one handler and returns the recorder.
func respond(t *testing.T, h router.Handler) *httptest.ResponseRecorder {
	t.Helper()
	r := router.New()
	r.Get("/x", h)
	return serve(t, r, http.MethodGet, "/x")
}

func TestTextWritesAPlainBody(t *testing.T) {
	rec := respond(t, func(*router.Ctx) (router.Response, error) {
		return router.Text(201, "made"), nil
	})
	if rec.Code != 201 || rec.Body.String() != "made" {
		t.Fatalf("gave %d %q", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("Content-Type is %q", ct)
	}
}

func TestJSONWritesAnEncodedBody(t *testing.T) {
	type thing struct {
		Name string `json:"name"`
	}
	rec := respond(t, func(*router.Ctx) (router.Response, error) {
		return router.JSON(200, thing{Name: "widget"}), nil
	})
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type is %q", ct)
	}
	var got thing
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("the body does not parse: %v", err)
	}
	if got.Name != "widget" {
		t.Fatalf("the body is %+v", got)
	}
}

func TestRedirectSetsTheLocation(t *testing.T) {
	rec := respond(t, func(*router.Ctx) (router.Response, error) {
		return router.Redirect(303, "/things/1"), nil
	})
	if rec.Code != 303 {
		t.Fatalf("gave %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/things/1" {
		t.Fatalf("Location is %q", loc)
	}
}

func TestNoContentWritesNoBody(t *testing.T) {
	rec := respond(t, func(*router.Ctx) (router.Response, error) {
		return router.NoContent(), nil
	})
	if rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("gave %d with a body of %d bytes", rec.Code, rec.Body.Len())
	}
}

func TestStatusWritesTheStandardText(t *testing.T) {
	rec := respond(t, func(*router.Ctx) (router.Response, error) {
		return router.Status(404), nil
	})
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), "Not Found") {
		t.Fatalf("gave %d %q", rec.Code, rec.Body.String())
	}
}

func TestHTMLSetsTheContentType(t *testing.T) {
	rec := respond(t, func(*router.Ctx) (router.Response, error) {
		return router.HTML(200, "<p>hello</p>"), nil
	})
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type is %q", ct)
	}
	if rec.Body.String() != "<p>hello</p>" {
		t.Fatalf("the body is %q", rec.Body.String())
	}
}

func TestAResponseReportsItsStatusBeforeItWrites(t *testing.T) {
	// The transaction middleware reads Status to decide the commit before
	// anything reaches the client.
	for _, tc := range []struct {
		name string
		res  router.Response
		want int
	}{
		{"text", router.Text(201, "x"), 201},
		{"json", router.JSON(200, nil), 200},
		{"redirect", router.Redirect(303, "/x"), 303},
		{"no content", router.NoContent(), 204},
		{"status", router.Status(422), 422},
		{"html", router.HTML(200, "x"), 200},
	} {
		if got := tc.res.Status(); got != tc.want {
			t.Fatalf("%s reports %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestANilResponseWithNoErrorIs204(t *testing.T) {
	rec := respond(t, func(*router.Ctx) (router.Response, error) { return nil, nil })
	if rec.Code != 204 {
		t.Fatalf("gave %d, want 204", rec.Code)
	}
}

func TestAResponseCanSetAHeader(t *testing.T) {
	rec := respond(t, func(c *router.Ctx) (router.Response, error) {
		c.Header().Set("X-Thing", "yes")
		return router.NoContent(), nil
	})
	if rec.Header().Get("X-Thing") != "yes" {
		t.Fatal("the header did not reach the response")
	}
}
