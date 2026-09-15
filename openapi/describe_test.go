package openapi_test

import (
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

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
	doc, err := openapi.Describe("blog", "1.0.0", rep.Routes, router.API{})
	if err != nil {
		t.Fatalf("Describe returned %v", err)
	}
	return doc
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
	doc, err := openapi.Describe("blog", "1.0.0", rep.Routes, router.API{})
	if err != nil {
		t.Fatalf("Describe returned %v", err)
	}
	if names := doc.PathNames(); len(names) != 1 || names[0] != "/posts" {
		t.Fatalf("the description holds %v", names)
	}
}

// Documented states what a model means. The description carries the sentence,
// and a type states it in Go and not in a comment that nothing reads.
type Note struct {
	// Body carries the tags that state the prose and the value.
	Body string `json:"body" doc:"The text of the note" example:"A note"`
	// Count shows that a number reaches the document as a number.
	Count int `json:"count" example:"7" default:"1"`
	// Secret shows that a member of a request never reaches an answer.
	Secret string `json:"secret,omitempty" openapi:"writeOnly"`
	// When shows that a time reaches the document as a string.
	When time.Time `json:"when"`
	// Meta shows that a map states the type of its value.
	Meta map[string]int `json:"meta,omitempty"`
	// Rank shows that a pointer that the answer carries reaches null.
	Rank *int `json:"rank"`
}

// Doc states what the model means.
func (Note) Doc() string { return "One note of a reader" }

func note(_ *router.Ctx, _ listInput) (Note, error) { return Note{}, nil }

// The tags of a field state the prose, the example and the value that the
// service uses when the request carries none.
func TestTheTagsOfAFieldReachTheDescription(t *testing.T) {
	r := router.New()
	router.Get(r, "/notes", note)
	rep, _ := r.Report()
	doc, err := openapi.Describe("blog", "1.0.0", rep.Routes, router.API{})
	if err != nil {
		t.Fatalf("Describe returned %v", err)
	}

	schema := doc.Components.Schemas["Note"]
	if schema.Description != "One note of a reader" {
		t.Fatalf("the model states %q", schema.Description)
	}
	body := schema.Properties["body"]
	if body.Description != "The text of the note" || body.Example != "A note" {
		t.Fatalf("the member states %+v", body)
	}
	// A number reaches the document as a number, not as a string.
	if count := schema.Properties["count"]; count.Example != int64(7) || count.Default != int64(1) {
		t.Fatalf("the number states %+v", count)
	}
	if secret := schema.Properties["secret"]; !secret.WriteOnly {
		t.Fatalf("the secret states %+v", secret)
	}
	// A time writes a string of RFC 3339, and not an object of its fields.
	when := schema.Properties["when"]
	if when.Type != "string" || when.Format != "date-time" {
		t.Fatalf("the time states %+v", when)
	}
	// A map states the type of its value.
	meta := schema.Properties["meta"]
	value, ok := meta.AdditionalProperties.(*openapi.Schema)
	if !ok || value.Type != "integer" {
		t.Fatalf("the map states %+v", meta)
	}
	// A pointer that the answer carries reaches null.
	rank := schema.Properties["rank"]
	list, ok := rank.Type.([]any)
	if !ok || len(list) != 2 || list[1] != "null" {
		t.Fatalf("the pointer states %+v", rank)
	}
}

// Two answers of one status state that the answer carries one of two shapes.
func TestTwoAnswersOfOneStatusStateOneOf(t *testing.T) {
	r := router.New()
	router.Post(r, "/posts", create,
		router.Answers[View](http.StatusConflict, "the title is taken"),
		router.Answers[PostList](http.StatusConflict, "the titles are taken"),
		router.AnswerHeader(http.StatusCreated, "Location", "the address of the new post"))
	rep, _ := r.Report()
	doc, err := openapi.Describe("blog", "1.0.0", rep.Routes, router.API{})
	if err != nil {
		t.Fatalf("Describe returned %v", err)
	}

	op, _ := doc.Operation(http.MethodPost, "/posts")
	one := op.Responses["409"].Content["application/json"].Schema
	if len(one.OneOf) != 2 {
		t.Fatalf("the answer states %+v", one)
	}
	// A header of an answer reaches the description.
	if _, ok := op.Responses["201"].Headers["Location"]; !ok {
		t.Fatalf("the answer states no header: %+v", op.Responses["201"])
	}
}

// The API states the facts that stand above every route.
func TestTheAPIStatesItsInfoAndItsSchemes(t *testing.T) {
	r := router.New(router.WithAPI(router.API{
		Title:       "Blog",
		Version:     "2.0.0",
		Description: "The service that holds the posts.",
		Servers:     []router.Server{{URL: "https://api.example.com", Description: "production"}},
		Security:    map[string]router.SecurityScheme{"bearer": router.BearerAuth("The token of a session.")},
		Require:     []string{"bearer"},
		Tags:        []router.Tag{{Name: "posts", Description: "The posts of the blog."}},
	}))
	router.Get(r, "/posts", list)
	router.Get(r, "/health", list, router.Public())
	rep, _ := r.Report()
	doc, err := openapi.Describe("ignored", "0.0.0", rep.Routes, r.API())
	if err != nil {
		t.Fatalf("Describe returned %v", err)
	}

	if doc.Info.Title != "Blog" || doc.Info.Version != "2.0.0" || doc.Info.Description == "" {
		t.Fatalf("the info states %+v", doc.Info)
	}
	if len(doc.Servers) != 1 || len(doc.Security) != 1 || len(doc.Tags) != 1 {
		t.Fatalf("the document states %+v", doc)
	}
	if _, ok := doc.Components.SecuritySchemes["bearer"]; !ok {
		t.Fatalf("the components hold no scheme: %+v", doc.Components)
	}
	// A public route states an empty list, so it stands outside the scheme.
	op, _ := doc.Operation(http.MethodGet, "/health")
	if op.Security == nil || len(*op.Security) != 0 {
		t.Fatalf("the public route states %+v", op.Security)
	}
}

// formInput reads a form, as a page of the ssr shape does.
type formInput struct {
	Title string `form:"title" validate:"required"`
}

func (i *formInput) Bind(_ *router.Ctx) error                 { return nil }
func (i *formInput) Validate(_ *router.Ctx, _ *router.Fields) {}

func send(_ *router.Ctx, _ formInput) (View, error) { return View{}, nil }

// The tags of the input state the media type that the route reads. A field
// with a form tag reads a form, and a field with a json tag reads JSON.
func TestTheTagsOfTheInputStateTheMediaType(t *testing.T) {
	r := router.New()
	router.Post(r, "/forms", send)
	router.Post(r, "/posts", create)
	rep, _ := r.Report()
	doc, err := openapi.Describe("blog", "1.0.0", rep.Routes, router.API{})
	if err != nil {
		t.Fatalf("Describe returned %v", err)
	}

	form, _ := doc.Operation(http.MethodPost, "/forms")
	if _, ok := form.RequestBody.Content[openapi.FormContent]; !ok {
		t.Fatalf("the form route reads %v", form.RequestBody.Content)
	}
	if _, ok := form.RequestBody.Content[openapi.JSONContent]; ok {
		t.Fatalf("the form route also reads JSON: %v", form.RequestBody.Content)
	}
	post, _ := doc.Operation(http.MethodPost, "/posts")
	if _, ok := post.RequestBody.Content[openapi.JSONContent]; !ok {
		t.Fatalf("the JSON route reads %v", post.RequestBody.Content)
	}
}

// QuestSummary is the part of the answer that a list and a detail share.
type QuestSummary struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// QuestDetail carries the summary and the body of one quest.
type QuestDetail struct {
	QuestSummary
	Body string `json:"body"`
}

type questInput struct {
	QuestSummary
	Note string `json:"note" validate:"required"`
}

func (i *questInput) Bind(_ *router.Ctx) error                 { return nil }
func (i *questInput) Validate(_ *router.Ctx, _ *router.Fields) {}

func detail(_ *router.Ctx, _ questInput) (QuestDetail, error) { return QuestDetail{}, nil }

// Go writes the fields of an embedded struct as members of the outer object,
// so the schema states them. A closed schema without them refuses the answer
// that the service writes.
func TestTheSchemaHoldsTheFieldsOfAnEmbeddedStruct(t *testing.T) {
	r := router.New()
	router.Post(r, "/quests", detail)
	rep, err := r.Report()
	if err != nil {
		t.Fatalf("the router holds a fault: %v", err)
	}
	doc, err := openapi.Describe("quests", "1.0.0", rep.Routes, router.API{})
	if err != nil {
		t.Fatalf("Describe returned %v", err)
	}

	schema, ok := doc.Components.Schemas["QuestDetail"]
	if !ok {
		t.Fatal("the components hold no QuestDetail")
	}
	for _, member := range []string{"id", "title", "body"} {
		if _, held := schema.Properties[member]; !held {
			t.Fatalf("QuestDetail holds no %q member: %v", member, schema.Properties)
		}
	}

	op, ok := doc.Operation(http.MethodPost, "/quests")
	if !ok {
		t.Fatal("the description holds no POST /quests")
	}
	body := op.RequestBody.Content[openapi.JSONContent].Schema
	for _, member := range []string{"id", "title", "note"} {
		if _, held := body.Properties[member]; !held {
			t.Fatalf("the request body holds no %q member: %v", member, body.Properties)
		}
	}
}

// The router answers 422 with the errors member beside the members of RFC
// 9457. The description states it, so a generated client reads the field
// errors and needs no cast.
func TestTheValidationAnswerStatesTheFieldErrors(t *testing.T) {
	doc := describe(t)

	op, ok := doc.Operation(http.MethodPost, "/posts")
	if !ok {
		t.Fatal("the description holds no POST /posts")
	}
	answer, ok := op.Responses["422"]
	if !ok {
		t.Fatal("POST /posts answers no 422")
	}
	ref := answer.Content[router.ProblemContentType].Schema.Ref
	if ref != "#/components/schemas/"+openapi.ValidationProblemSchema {
		t.Fatalf("the 422 answer names %q", ref)
	}
	schema, ok := doc.Components.Schemas[openapi.ValidationProblemSchema]
	if !ok {
		t.Fatal("the components hold no ValidationProblem")
	}
	if len(schema.AllOf) != 2 {
		t.Fatalf("ValidationProblem holds %d members of allOf", len(schema.AllOf))
	}
	errorsMember, held := schema.AllOf[1].Properties["errors"]
	if !held {
		t.Fatalf("ValidationProblem holds no errors member: %v", schema.AllOf[1].Properties)
	}
	value, ok := errorsMember.AdditionalProperties.(*openapi.Schema)
	if !ok || value.Type != "string" {
		t.Fatalf("the errors member holds %v", errorsMember.AdditionalProperties)
	}
}

// Two types of two packages can carry one name. The description holds one
// schema for each name, so the second would take the shape of the first. The
// walk reports the pair instead, and the fault names both types.
type collidingInput struct {
	Name string `json:"name"`
}

func (i *collidingInput) Bind(_ *router.Ctx) error                 { return nil }
func (i *collidingInput) Validate(_ *router.Ctx, _ *router.Fields) {}

// View collides with the View of this package, because both state the name
// View. The second states another name, so the description holds both.
type renamedView struct {
	Body string `json:"body"`
}

func (renamedView) SchemaName() string { return "View" }

func collide(_ *router.Ctx, _ collidingInput) (renamedView, error) { return renamedView{}, nil }

func TestTwoTypesOfOneSchemaNameAreAFault(t *testing.T) {
	r := router.New()
	router.Post(r, "/posts", create)
	router.Post(r, "/notes", collide)
	rep, err := r.Report()
	if err != nil {
		t.Fatalf("the router holds a fault: %v", err)
	}
	_, err = openapi.Describe("blog", "1.0.0", rep.Routes, router.API{})
	if err == nil {
		t.Fatal("the description accepted two types of one name")
	}
	for _, want := range []string{"View", "SchemaName", "renamedView"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the fault holds no %q: %v", want, err)
		}
	}
}

// A type that states its own schema name keeps the two apart, so the
// description holds one schema for each.
func TestAStatedSchemaNameKeepsTwoTypesApart(t *testing.T) {
	r := router.New()
	router.Post(r, "/notes", collide)
	rep, err := r.Report()
	if err != nil {
		t.Fatalf("the router holds a fault: %v", err)
	}
	doc, err := openapi.Describe("blog", "1.0.0", rep.Routes, router.API{})
	if err != nil {
		t.Fatalf("Describe returned %v", err)
	}
	if _, ok := doc.Components.Schemas["View"]; !ok {
		t.Fatalf("the components hold no View: %v", doc.Components.Schemas)
	}
}

// uploadInput carries a file and a text field, which is the shape of a form
// that a browser posts with an image.
type uploadInput struct {
	ID    string                `path:"id"`
	Note  string                `form:"note"`
	Image *multipart.FileHeader `form:"image" validate:"required"`
}

func (i *uploadInput) Bind(_ *router.Ctx) error                 { return nil }
func (i *uploadInput) Validate(_ *router.Ctx, _ *router.Fields) {}

func upload(_ *router.Ctx, _ uploadInput) (View, error) { return View{}, nil }

// A route that binds a file states a body of multipart/form-data, and the file
// stands in it as a string of bytes. A generator of a client writes the upload
// from that description.
func TestARouteWithAFileStatesAMultipartBody(t *testing.T) {
	r := router.New()
	router.Post(r, "/posts/{id}/image", upload)
	rep, err := r.Report()
	if err != nil {
		t.Fatalf("the router holds a fault: %v", err)
	}
	doc, err := openapi.Describe("blog", "1.0.0", rep.Routes, router.API{})
	if err != nil {
		t.Fatalf("Describe returned %v", err)
	}
	op, ok := doc.Operation(http.MethodPost, "/posts/{id}/image")
	if !ok {
		t.Fatal("the description holds no POST /posts/{id}/image")
	}
	if _, held := op.RequestBody.Content[openapi.JSONContent]; held {
		t.Fatal("the body states JSON, which carries no file")
	}
	media, held := op.RequestBody.Content[openapi.MultipartContent]
	if !held {
		t.Fatalf("the body states no multipart form: %v", op.RequestBody.Content)
	}
	image, held := media.Schema.Properties["image"]
	if !held {
		t.Fatalf("the body holds no image member: %v", media.Schema.Properties)
	}
	if image.Type != "string" || image.Format != "binary" {
		t.Fatalf("the image member is %v %q", image.Type, image.Format)
	}
	if _, held := media.Schema.Properties["note"]; !held {
		t.Fatal("the body holds no note member")
	}
	// The path parameter stays a parameter and no member of the body.
	if _, held := media.Schema.Properties["id"]; held {
		t.Fatal("the body holds the path parameter")
	}
}
