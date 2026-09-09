package openapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/alternayte/avero/openapi"
	"github.com/alternayte/avero/router"
)

type listInput struct {
	Search string `query:"q"`
}

func (i *listInput) Bind(_ *router.Ctx) error                 { return nil }
func (i *listInput) Validate(_ *router.Ctx, _ *router.Fields) {}

type createInput struct {
	Title string `json:"title" validate:"required,min=3,max=80"`
	Body  string `json:"body" validate:"required"`
}

func (i *createInput) Bind(_ *router.Ctx) error                 { return nil }
func (i *createInput) Validate(_ *router.Ctx, _ *router.Fields) {}

type showInput struct {
	ID string `path:"id" validate:"required,uuid"`
}

func (i *showInput) Bind(_ *router.Ctx) error                 { return nil }
func (i *showInput) Validate(_ *router.Ctx, _ *router.Fields) {}

// View is the answer of one post.
type View struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Note  string `json:"note,omitempty"`
}

// PostList holds every post.
type PostList struct {
	Posts []View `json:"posts"`
}

func list(_ *router.Ctx, _ listInput) (PostList, error) {
	return PostList{}, nil
}

func create(_ *router.Ctx, _ createInput) (View, error) {
	return View{}, nil
}

func remove(_ *router.Ctx, _ showInput) (router.NoBody, error) {
	return router.NoBody{}, nil
}

// describe builds the description of a small application.
func describe(t *testing.T) *openapi.Document {
	t.Helper()
	r := router.New()
	router.Get(r, "/posts", list, router.Summary("List every post"))
	router.Post(r, "/posts", create)
	router.Delete(r, "/posts/{id}", remove)

	rep, err := r.Report()
	if err != nil {
		t.Fatalf("the router holds a fault: %v", err)
	}
	return openapi.Describe("blog", "1.0.0", rep.Routes, nil)
}

// The description names the schema of the answer, and the components carry it.
// The type comes from the signature of the handler, so no comment states it.
func TestTheDescriptionCarriesTheAnswerOfEachRoute(t *testing.T) {
	doc := describe(t)

	op, ok := doc.Operation(http.MethodGet, "/posts")
	if !ok {
		t.Fatalf("the description holds no GET /posts: %v", doc.PathNames())
	}
	if op.Summary != "List every post" {
		t.Fatalf("the summary is %q", op.Summary)
	}
	ref := op.Responses["200"].Content["application/json"].Schema.Ref
	if ref != "#/components/schemas/PostList" {
		t.Fatalf("the answer names %q", ref)
	}
	// Every schema that a reference names must stand in the components.
	for _, name := range []string{"PostList", "View", openapi.ProblemSchema} {
		if _, ok := doc.Components.Schemas[name]; !ok {
			t.Fatalf("the components hold no %s: %v", name, doc.Components.Schemas)
		}
	}
	// A field with omitempty is not required. Every other field stands in the
	// answer.
	view := doc.Components.Schemas["View"]
	if len(view.Required) != 2 {
		t.Fatalf("the required members are %v", view.Required)
	}
}

// A POST states 201, and the body of the request carries the rules of the
// validate tag.
func TestTheDescriptionCarriesTheBodyAndItsRules(t *testing.T) {
	doc := describe(t)
	op, _ := doc.Operation(http.MethodPost, "/posts")

	if _, ok := op.Responses["201"]; !ok {
		t.Fatalf("the answers are %v", op.Responses)
	}
	if op.RequestBody == nil {
		t.Fatal("the operation carries no body")
	}
	body := op.RequestBody.Content["application/json"].Schema
	title := body.Properties["title"]
	if title.MinLength == nil || *title.MinLength != 3 || title.MaxLength == nil || *title.MaxLength != 80 {
		t.Fatalf("the title states %+v", title)
	}
	if len(body.Required) != 2 {
		t.Fatalf("the required members are %v", body.Required)
	}
}

// A path parameter reaches the description with the format that its rule
// states, and an answer with no body names no schema.
func TestTheDescriptionCarriesTheParametersAndTheEmptyAnswer(t *testing.T) {
	doc := describe(t)
	op, _ := doc.Operation(http.MethodDelete, "/posts/{id}")

	if len(op.Parameters) != 1 {
		t.Fatalf("the parameters are %+v", op.Parameters)
	}
	p := op.Parameters[0]
	if p.Name != "id" || p.In != "path" || !p.Required || p.Schema.Format != "uuid" {
		t.Fatalf("the parameter is %+v", p)
	}
	if len(op.Responses["204"].Content) != 0 {
		t.Fatalf("the empty answer carries %v", op.Responses["204"].Content)
	}
}

// Every route states the fault of the service as a problem document, so a
// client reads one error shape for the whole API.
func TestEveryRouteStatesTheProblemAnswer(t *testing.T) {
	doc := describe(t)
	for _, path := range doc.PathNames() {
		for method, op := range doc.Paths[path] {
			answer, ok := op.Responses["500"]
			if !ok {
				t.Fatalf("%s %s states no 500", method, path)
			}
			if _, ok := answer.Content[router.ProblemContentType]; !ok {
				t.Fatalf("%s %s answers no problem document: %v", method, path, answer)
			}
		}
	}
	// The description must parse as JSON, because a generator reads it.
	if !json.Valid([]byte(doc.String())) {
		t.Fatalf("the description is not JSON:\n%s", doc.String())
	}
}

// Two runs write one document. A generator of a client reads the file, so a
// run that reorders the members would write a change that means nothing. See
// AN-4.
func TestTheDocumentIsStableAndParses(t *testing.T) {
	first := describe(t)
	second := describe(t)
	if first.String() != second.String() {
		t.Fatal("two runs wrote two documents")
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(first.String()), &doc); err != nil {
		t.Fatalf("the document does not parse: %v", err)
	}
	if doc["openapi"] != openapi.Version || doc["paths"] == nil || doc["info"] == nil {
		t.Fatalf("the document holds %v", doc)
	}
	if got := doc["info"].(map[string]any)["title"]; got != "blog" {
		t.Fatalf("the document names %v", got)
	}
}

// A route that the application registers with no type states no operation, so
// the description carries the routes of the API and no asset handler.
func TestARouteWithNoTypeStatesNoOperation(t *testing.T) {
	r := router.New()
	r.Get("/{$}", func(_ *router.Ctx) (router.Response, error) {
		return router.NoContent(), nil
	})
	router.Get(r, "/posts", list)

	rep, err := r.Report()
	if err != nil {
		t.Fatalf("the router holds a fault: %v", err)
	}
	doc := openapi.Describe("blog", "1.0.0", rep.Routes, nil)
	if names := doc.PathNames(); len(names) != 1 || names[0] != "/posts" {
		t.Fatalf("the description holds %v", names)
	}
}
