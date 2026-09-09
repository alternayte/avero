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

// answer runs one route and returns the recorder.
func answer(t *testing.T, h router.Handler, method, target string, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := router.New()
	r.Handle(method, target, h)
	handler, err := r.Handler()
	if err != nil {
		t.Fatalf("the router holds a fault: %v", err)
	}
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// A handler that returns a Problem answers the status of the problem, and the
// body is a problem document. The client reads one shape for every failure.
func TestAProblemReachesTheClientAsADocument(t *testing.T) {
	rec := answer(t, func(_ *router.Ctx) (router.Response, error) {
		return nil, router.NotFound("post", "7")
	}, http.MethodGet, "/posts", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != router.ProblemContentType {
		t.Fatalf("the content type is %q, want %q", got, router.ProblemContentType)
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("the document does not parse: %v\n%s", err, rec.Body.String())
	}
	if doc["status"] != float64(http.StatusNotFound) || doc["title"] != "Not Found" {
		t.Fatalf("the document holds %v", doc)
	}
	if !strings.Contains(doc["detail"].(string), "post 7") {
		t.Fatalf("the detail is %q", doc["detail"])
	}
	// RFC 9457 asks for the instance, so a report names the request.
	if doc["instance"] != "/posts" {
		t.Fatalf("the instance is %v", doc["instance"])
	}
}

// An error that carries no problem is a fault of the service. The client reads
// 500 and nothing of the message, which can name an internal detail.
func TestAPlainErrorAnswersFiveHundredAndHidesItsMessage(t *testing.T) {
	rec := answer(t, func(_ *router.Ctx) (router.Response, error) {
		return nil, errors.New("the password of the database is wrong")
	}, http.MethodGet, "/posts", "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "password") {
		t.Fatalf("the answer carries the message of the error:\n%s", rec.Body.String())
	}
}

// A store returns its own error, and a handler wraps it. errors.Is reaches the
// problem, and errors.As reaches the cause, so a caller reads both.
func TestAWrappedProblemCarriesItsCause(t *testing.T) {
	cause := errors.New("the row is locked")
	err := error(router.Conflict("the post is in use").Wrap(cause))

	if !errors.Is(err, router.ErrConflict) {
		t.Fatal("errors.Is does not reach the conflict")
	}
	if errors.Is(err, router.ErrNotFound) {
		t.Fatal("errors.Is answers for another status")
	}
	if !errors.Is(err, cause) {
		t.Fatal("errors.Is does not reach the cause")
	}
	if got := router.ProblemOf(err); got.Code != http.StatusConflict {
		t.Fatalf("the problem is %v", got)
	}
}

// A validation fault answers one problem document, and the field messages ride
// in the errors member beside the members of RFC 9457.
func TestAValidationFaultAnswersAProblemWithTheFields(t *testing.T) {
	rec := answer(t, router.In(func(_ *router.Ctx, _ probeInput) (router.Response, error) {
		return router.NoContent(), nil
	}), http.MethodPost, "/probe", `{}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422:\n%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != router.ProblemContentType {
		t.Fatalf("the content type is %q, want %q", got, router.ProblemContentType)
	}
	var doc struct {
		Status int               `json:"status"`
		Title  string            `json:"title"`
		Errors map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("the document does not parse: %v\n%s", err, rec.Body.String())
	}
	if doc.Status != http.StatusUnprocessableEntity || doc.Title == "" {
		t.Fatalf("the document holds %+v", doc)
	}
	if doc.Errors["title"] == "" {
		t.Fatalf("the document names no field:\n%s", rec.Body.String())
	}
}

// probeInput fails validation with no field set.
type probeInput struct {
	Title string `json:"title"`
}

func (p *probeInput) Bind(_ *router.Ctx) error { return nil }

func (p *probeInput) Validate(_ *router.Ctx, f *router.Fields) {
	if p.Title == "" {
		f.Add("title", "the title is required")
	}
}
