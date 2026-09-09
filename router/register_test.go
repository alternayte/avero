package router_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alternayte/avero/router"
)

type showInput struct {
	ID string `path:"id"`
}

func (s *showInput) Bind(c *router.Ctx) error {
	// The generated Bind reads the path. This one stands for it.
	s.ID = c.PathValue("id")
	return nil
}

func (s *showInput) Validate(_ *router.Ctx, _ *router.Fields) {}

type postView struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func show(_ *router.Ctx, in showInput) (postView, error) {
	return postView{ID: in.ID, Title: "A title"}, nil
}

func remove(_ *router.Ctx, _ showInput) (router.NoBody, error) {
	return router.NoBody{}, nil
}

// A typed registration needs no wrapper and no comment. A handler returns the
// thing that it answers, as an ordinary Go function does, and the types come
// from its signature.
func TestATypedRouteRecordsItsTypes(t *testing.T) {
	r := router.New()
	router.Get(r, "/posts/{id}", show)

	rep, err := r.Report()
	if err != nil {
		t.Fatalf("the router holds a fault: %v", err)
	}
	route, ok := rep.Route(http.MethodGet, "/posts/{id}")
	if !ok {
		t.Fatalf("the report holds no route: %+v", rep.Routes)
	}
	if route.Op == nil {
		t.Fatal("the route states no operation")
	}
	if got := route.Op.Input().Name(); got != "showInput" {
		t.Fatalf("the input is %q", got)
	}
	if len(route.Op.Answers) != 1 || route.Op.Answers[0].Code != http.StatusOK {
		t.Fatalf("the answers are %+v", route.Op.Answers)
	}
	if got := route.Op.Answers[0].Body().Name(); got != "postView" {
		t.Fatalf("the body is %q", got)
	}
	// The fault of a route names the file of the person who registered it.
	if !strings.HasSuffix(route.File, "register_test.go") {
		t.Fatalf("the route names %s", route.File)
	}
}

// A POST answers 201, because it makes a thing.
func TestATypedPostAnswersCreated(t *testing.T) {
	r := router.New()
	router.Post(r, "/posts", show)
	rep, _ := r.Report()
	route, _ := rep.Route(http.MethodPost, "/posts")
	if route.Op.Answers[0].Code != http.StatusCreated {
		t.Fatalf("the answer is %d, want 201", route.Op.Answers[0].Code)
	}
}

// A handler with no body answers 204, and the answer names no type.
func TestAnAnswerWithNoBodyStatesNoType(t *testing.T) {
	r := router.New()
	router.Delete(r, "/posts/{id}", remove)
	rep, _ := r.Report()
	route, _ := rep.Route(http.MethodDelete, "/posts/{id}")
	if route.Op.Answers[0].Code != http.StatusNoContent {
		t.Fatalf("the answer is %d, want 204", route.Op.Answers[0].Code)
	}
	if route.Op.Answers[0].Body() != nil {
		t.Fatalf("the answer names %v", route.Op.Answers[0].Body())
	}
}

// An option states a case that the signature cannot carry, such as a second
// status or a tag.
func TestAnOptionStatesAnotherAnswer(t *testing.T) {
	r := router.New()
	router.Post(r, "/posts", show,
		router.Answers[postView](http.StatusConflict, "the title is taken"),
		router.Tags("posts"),
		router.Deprecated())
	rep, _ := r.Report()
	route, _ := rep.Route(http.MethodPost, "/posts")
	if len(route.Op.Answers) != 2 || route.Op.Answers[1].Code != http.StatusConflict {
		t.Fatalf("the answers are %+v", route.Op.Answers)
	}
	if !route.Op.Deprecated || len(route.Op.Tags) != 1 {
		t.Fatalf("the operation is %+v", route.Op)
	}
}

// The typed route serves the request, so the registration changes the shape of
// the code and nothing of the behaviour.
func TestATypedRouteAnswersTheRequest(t *testing.T) {
	r := router.New()
	router.Get(r, "/posts/{id}", show)
	handler, err := r.Handler()
	if err != nil {
		t.Fatalf("the router holds a fault: %v", err)
	}
	rec := requestTo(t, handler, http.MethodGet, "/posts/7")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"id":"7"`) {
		t.Fatalf("the body is %s", rec.Body.String())
	}
}

// requestTo performs one request against the handler.
func requestTo(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// The method of the route states the status, and Ctx.Status names another.
func TestAHandlerNamesAnotherStatus(t *testing.T) {
	r := router.New()
	router.Post(r, "/posts", func(c *router.Ctx, _ showInput) (postView, error) {
		c.Status(http.StatusAccepted)
		return postView{ID: "7"}, nil
	})
	handler, err := r.Handler()
	if err != nil {
		t.Fatalf("the router holds a fault: %v", err)
	}
	if rec := requestTo(t, handler, http.MethodPost, "/posts"); rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
}

// A handler that answers no body writes the status and nothing else.
func TestAnEmptyAnswerWritesNoBody(t *testing.T) {
	r := router.New()
	router.Delete(r, "/posts/{id}", remove)
	handler, err := r.Handler()
	if err != nil {
		t.Fatalf("the router holds a fault: %v", err)
	}
	rec := requestTo(t, handler, http.MethodDelete, "/posts/7")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status = %d and the body holds %q", rec.Code, rec.Body.String())
	}
}

// The generated map states the summary of a handler, and an option of a route
// wins over it.
func TestASummaryComesFromTheGeneratedMap(t *testing.T) {
	r := router.New(router.WithSummaries(map[string]string{
		"show":   "Answers one post",
		"remove": "Removes one post",
	}))
	router.Get(r, "/posts/{id}", show)
	router.Delete(r, "/posts/{id}", remove, router.Summary("Delete a post for good"))

	rep, err := r.Report()
	if err != nil {
		t.Fatalf("the router holds a fault: %v", err)
	}
	got, _ := rep.Route(http.MethodGet, "/posts/{id}")
	if got.Op.Summary != "Answers one post" {
		t.Fatalf("the summary is %q", got.Op.Summary)
	}
	// The option of the route wins, because a person wrote it at the route.
	removed, _ := rep.Route(http.MethodDelete, "/posts/{id}")
	if removed.Op.Summary != "Delete a post for good" {
		t.Fatalf("the summary is %q", removed.Op.Summary)
	}
}
