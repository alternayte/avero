package openapi_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/avero/openapi"
)

// application writes a small application and returns its directory.
func application(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "go.mod", "module blog\n\ngo 1.26.2\n")
	write(t, dir, "wire.go", `package main

import "github.com/alternayte/avero"

func wire(r *avero.Router) {
	r.Get("/{$}", func(c *avero.Ctx) (avero.Response, error) {
		return avero.NoContent(), nil
	})
}
`)
	write(t, filepath.Join(dir, "internal", "features", "posts"), "module.go", `package posts

import "github.com/alternayte/avero"

type Module struct{}

func (m *Module) Routes(r *avero.Router) {
	r.Get("/posts", avero.In(m.List))
	r.Post("/posts", avero.In(m.Create))
	r.Get("/posts/{id}", avero.In(m.Show))
	r.Delete("/posts/{id}", avero.In(m.Delete))
}
`)
	write(t, filepath.Join(dir, "internal", "features", "posts"), "input.go", `package posts

import "time"

type ListInput struct {
	Search string `+"`query:\"q\" validate:\"max=64\"`"+`
	Page   int    `+"`query:\"page\" validate:\"min=1,max=100\"`"+`
}

type CreateInput struct {
	Title    string    `+"`json:\"title\" validate:\"required,min=3,max=80\"`"+`
	Email    string    `+"`json:\"email\" validate:\"required,email\"`"+`
	Role     string    `+"`json:\"role\" validate:\"oneof=admin member\"`"+`
	Notify   bool      `+"`json:\"notify\"`"+`
	StartsAt time.Time `+"`json:\"starts_at\"`"+`
	Key      string    `+"`header:\"X-Api-Key\" validate:\"required\"`"+`
}

type ShowInput struct {
	ID string `+"`path:\"id\" validate:\"required,uuid\"`"+`
}

type DeleteInput struct {
	ID string `+"`path:\"id\" validate:\"required\"`"+`
}
`)
	write(t, filepath.Join(dir, "internal", "features", "posts"), "handlers.go", `package posts

import "github.com/alternayte/avero"

// List answers every post. It reads the query of the request.
func (m *Module) List(c *avero.Ctx, in ListInput) (avero.Response, error) {
	return avero.NoContent(), nil
}

// Create writes one post.
func (m *Module) Create(c *avero.Ctx, in CreateInput) (avero.Response, error) {
	return avero.NoContent(), nil
}

// Show answers one post.
func (m *Module) Show(c *avero.Ctx, in ShowInput) (avero.Response, error) {
	return avero.NoContent(), nil
}

// Delete removes one post.
func (m *Module) Delete(c *avero.Ctx, in DeleteInput) (avero.Response, error) {
	return avero.NoContent(), nil
}
`)
	// A store carries two parameters and two results as well, so the reader
	// must not read it as a handler.
	write(t, filepath.Join(dir, "internal", "features", "posts"), "store.go", `package posts

import "context"

type Store struct{}

// List returns the rows of the table.
func (s *Store) List(ctx context.Context, search string) ([]string, error) {
	return nil, nil
}
`)
	return dir
}

// write puts one file into a directory.
func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
}

func TestGenerateReadsEveryRouteOfTheApplication(t *testing.T) {
	doc, err := openapi.Generate(openapi.Options{Dir: application(t)})
	if err != nil {
		t.Fatalf("Generate returned %v, want nil", err)
	}
	if doc.OpenAPI != openapi.Version {
		t.Fatalf("the version is %q, want %s", doc.OpenAPI, openapi.Version)
	}
	if doc.Info.Title != "blog" {
		t.Fatalf("the title is %q, want the name of the module", doc.Info.Title)
	}
	want := []string{"/", "/posts", "/posts/{id}"}
	if strings.Join(doc.PathNames(), ",") != strings.Join(want, ",") {
		t.Fatalf("the paths are %v, want %v", doc.PathNames(), want)
	}
	for _, tc := range []struct{ method, path, id string }{
		{"GET", "/posts", "posts.List"},
		{"POST", "/posts", "posts.Create"},
		{"GET", "/posts/{id}", "posts.Show"},
		{"DELETE", "/posts/{id}", "posts.Delete"},
		{"GET", "/", "app.get_root"},
	} {
		op, ok := doc.Operation(tc.method, tc.path)
		if !ok {
			t.Fatalf("the document holds no %s %s", tc.method, tc.path)
		}
		if op.OperationID != tc.id {
			t.Fatalf("the operation of %s %s is %q, want %q", tc.method, tc.path, op.OperationID, tc.id)
		}
	}
}

func TestTheQueryAndThePathReachTheParameters(t *testing.T) {
	doc, err := openapi.Generate(openapi.Options{Dir: application(t)})
	if err != nil {
		t.Fatalf("Generate returned %v, want nil", err)
	}

	list, _ := doc.Operation("GET", "/posts")
	names := map[string]openapi.Parameter{}
	for _, p := range list.Parameters {
		names[p.Name] = p
	}
	if names["q"].In != "query" || names["q"].Schema.Type != "string" {
		t.Fatalf("the parameter q is %+v", names["q"])
	}
	if names["q"].Schema.MaxLength == nil || *names["q"].Schema.MaxLength != 64 {
		t.Fatalf("the rule max does not reach the schema: %+v", names["q"].Schema)
	}
	if names["page"].Schema.Type != "integer" || names["page"].Schema.Minimum == nil || *names["page"].Schema.Minimum != 1 {
		t.Fatalf("the parameter page is %+v", names["page"].Schema)
	}
	if list.RequestBody != nil {
		t.Fatal("a GET carries a body")
	}

	show, _ := doc.Operation("GET", "/posts/{id}")
	if len(show.Parameters) != 1 {
		t.Fatalf("the operation holds %d parameters", len(show.Parameters))
	}
	id := show.Parameters[0]
	if id.In != "path" || !id.Required || id.Schema.Format != "uuid" {
		t.Fatalf("the parameter id is %+v", id)
	}
}

func TestTheBodyCarriesTheFieldsAndTheRules(t *testing.T) {
	doc, err := openapi.Generate(openapi.Options{Dir: application(t)})
	if err != nil {
		t.Fatalf("Generate returned %v, want nil", err)
	}
	create, _ := doc.Operation("POST", "/posts")
	if create.RequestBody == nil {
		t.Fatal("the operation carries no body")
	}
	schema := create.RequestBody.Content["application/json"].Schema
	if schema.Type != "object" {
		t.Fatalf("the body is %+v", schema)
	}
	if got := schema.Properties["title"]; got.Type != "string" || got.MinLength == nil || *got.MinLength != 3 {
		t.Fatalf("the field title is %+v", got)
	}
	if got := schema.Properties["email"]; got.Format != "email" {
		t.Fatalf("the field email is %+v", got)
	}
	if got := schema.Properties["role"]; strings.Join(got.Enum, ",") != "admin,member" {
		t.Fatalf("the field role is %+v", got)
	}
	if got := schema.Properties["notify"]; got.Type != "boolean" {
		t.Fatalf("the field notify is %+v", got)
	}
	if got := schema.Properties["starts_at"]; got.Type != "string" || got.Format != "date-time" {
		t.Fatalf("the field starts_at is %+v", got)
	}
	if strings.Join(schema.Required, ",") != "email,title" {
		t.Fatalf("the required fields are %v", schema.Required)
	}
	// A header is a parameter, and it never reaches the body.
	if _, ok := schema.Properties["X-Api-Key"]; ok {
		t.Fatalf("the header reached the body: %+v", schema.Properties)
	}
	found := false
	for _, p := range create.Parameters {
		if p.Name == "X-Api-Key" && p.In == "header" && p.Required {
			found = true
		}
	}
	if !found {
		t.Fatalf("the header is no parameter: %+v", create.Parameters)
	}
}

func TestTheAnswersStateTheFaultsThatTheRouterWrites(t *testing.T) {
	doc, err := openapi.Generate(openapi.Options{Dir: application(t)})
	if err != nil {
		t.Fatalf("Generate returned %v, want nil", err)
	}
	create, _ := doc.Operation("POST", "/posts")
	for _, code := range []string{"200", "201", "400", "422", "500"} {
		if _, ok := create.Responses[code]; !ok {
			t.Fatalf("the operation states no %s: %v", code, create.Responses)
		}
	}
	root, _ := doc.Operation("GET", "/")
	if _, ok := root.Responses["422"]; ok {
		t.Fatal("a route with no input states a validation fault")
	}
}

func TestTheSummaryComesFromTheCommentOfTheHandler(t *testing.T) {
	doc, err := openapi.Generate(openapi.Options{Dir: application(t)})
	if err != nil {
		t.Fatalf("Generate returned %v, want nil", err)
	}
	list, _ := doc.Operation("GET", "/posts")
	if list.Summary != "answers every post" {
		t.Fatalf("the summary is %q", list.Summary)
	}
}

func TestTheDocumentParsesAndItsOrderIsStable(t *testing.T) {
	dir := application(t)
	first, err := openapi.Generate(openapi.Options{Dir: dir, Server: "https://api.example.com"})
	if err != nil {
		t.Fatalf("Generate returned %v, want nil", err)
	}
	second, err := openapi.Generate(openapi.Options{Dir: dir, Server: "https://api.example.com"})
	if err != nil {
		t.Fatalf("Generate returned %v, want nil", err)
	}
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
	servers, _ := doc["servers"].([]any)
	if len(servers) != 1 {
		t.Fatalf("the document holds %v servers", servers)
	}
}

func TestGenerateStatesTheRepairForADirectoryWithNoRoute(t *testing.T) {
	_, err := openapi.Generate(openapi.Options{Dir: t.TempDir()})
	if err == nil {
		t.Fatal("Generate returned nil, want a fault")
	}
	if !strings.Contains(err.Error(), "→") {
		t.Fatalf("the fault states no repair: %v", err)
	}
}

func TestTheDirectiveStatesTheAnswerAndItsSchema(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "go.mod", "module blog\n\ngo 1.26.2\n")
	write(t, filepath.Join(dir, "internal", "features", "tasks"), "module.go", `package tasks

import "github.com/alternayte/avero"

type Module struct{}

func (m *Module) Routes(r *avero.Router) {
	r.Get("/api/tasks", avero.In(m.List))
	r.Post("/api/tasks", avero.In(m.Create))
	r.Delete("/api/tasks/{id}", avero.In(m.Delete))
}
`)
	write(t, filepath.Join(dir, "internal", "features", "tasks"), "handlers.go", `package tasks

import "github.com/alternayte/avero"

type Task struct {
	ID    string `+"`json:\"id\"`"+`
	Title string `+"`json:\"title\"`"+`
	Note  string `+"`json:\"note,omitempty\"`"+`
}

type TaskList struct {
	Tasks []Task `+"`json:\"tasks\"`"+`
	Total int    `+"`json:\"total\"`"+`
}

type ListInput struct{}

type CreateInput struct {
	Title string `+"`json:\"title\" validate:\"required\"`"+`
}

type DeleteInput struct {
	ID string `+"`path:\"id\" validate:\"required\"`"+`
}

// List answers every task.
//
//avero:response 200 TaskList
func (m *Module) List(c *avero.Ctx, in ListInput) (avero.Response, error) {
	return avero.NoContent(), nil
}

// Create writes one task.
//
//avero:response 201 Task
func (m *Module) Create(c *avero.Ctx, in CreateInput) (avero.Response, error) {
	return avero.NoContent(), nil
}

// Delete removes one task.
//
//avero:response 204
func (m *Module) Delete(c *avero.Ctx, in DeleteInput) (avero.Response, error) {
	return avero.NoContent(), nil
}
`)

	doc, err := openapi.Generate(openapi.Options{Dir: dir})
	if err != nil {
		t.Fatalf("Generate returned %v, want nil", err)
	}

	list, _ := doc.Operation("GET", "/api/tasks")
	answer, ok := list.Responses["200"]
	if !ok || answer.Content["application/json"].Schema.Ref != "#/components/schemas/TaskList" {
		t.Fatalf("the answer of the list is %+v", list.Responses)
	}
	// A directive states the answer of the handler. The faults that the
	// router writes stand beside it, because the router writes them whatever
	// the handler states.
	if _, ok := list.Responses["500"]; !ok {
		t.Fatalf("the answers hold no fault of the router: %v", list.Responses)
	}

	create, _ := doc.Operation("POST", "/api/tasks")
	if create.Responses["201"].Content["application/json"].Schema.Ref != "#/components/schemas/Task" {
		t.Fatalf("the answer of the create is %+v", create.Responses)
	}

	remove, _ := doc.Operation("DELETE", "/api/tasks/{id}")
	if answer, ok := remove.Responses["204"]; !ok || len(answer.Content) != 0 {
		t.Fatalf("the answer of the delete is %+v", remove.Responses)
	}

	if doc.Components == nil {
		t.Fatal("the document holds no components")
	}
	task, ok := doc.Components.Schemas["Task"]
	if !ok {
		t.Fatalf("the components hold %v", doc.Components.Schemas)
	}
	if task.Properties["id"].Type != "string" || task.Properties["title"].Type != "string" {
		t.Fatalf("the schema of Task is %+v", task)
	}
	// Go writes every field, so each one is required. A tag with omitempty
	// states the one case that it does not.
	if strings.Join(task.Required, ",") != "id,title" {
		t.Fatalf("the required fields of Task are %v", task.Required)
	}

	taskList, ok := doc.Components.Schemas["TaskList"]
	if !ok {
		t.Fatal("the components hold no TaskList")
	}
	tasks := taskList.Properties["tasks"]
	if tasks.Type != "array" || tasks.Items == nil || tasks.Items.Ref != "#/components/schemas/Task" {
		t.Fatalf("the field tasks is %+v", tasks)
	}
	if taskList.Properties["total"].Type != "integer" {
		t.Fatalf("the field total is %+v", taskList.Properties["total"])
	}
	// The summary holds no directive.
	if strings.Contains(list.Summary, "avero:") {
		t.Fatalf("the summary holds a directive: %q", list.Summary)
	}
}
