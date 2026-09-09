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

func show(_ *router.Ctx, in showInput) (router.Result[postView], error) {
	return router.OK(postView{ID: in.ID, Title: "A title"}), nil
}

func remove(_ *router.Ctx, _ showInput) (router.Result[router.NoBody], error) {
	return router.Done(), nil
}

// A typed registration needs no wrapper and no comment. The types of the input
// and of the body come from the signature, so the compiler holds them and the
// description of the API reads them.
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
