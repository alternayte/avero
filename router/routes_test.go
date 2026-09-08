package router_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/alternayte/avero/router"
)

// reportRouter builds a router that exercises every row shape.
func reportRouter(t *testing.T) *router.Router {
	t.Helper()
	log, _ := captureLogger()
	r := router.New()
	r.Use(router.RequestID(), router.Recover(log))
	r.Get("/things", ok("list"))
	r.Group("/api", func(g *router.Router) {
		g.Use(router.CSRF(secret))
		g.Post("/things", ok("create"))
	})
	r.Mount("/admin", http.NotFoundHandler())
	return r
}

func TestTheReportHoldsOneRowForEachRoute(t *testing.T) {
	rep, err := reportRouter(t).Report()
	if err != nil {
		t.Fatalf("Report returned an error: %v", err)
	}
	if len(rep.Routes) != 3 {
		t.Fatalf("the report holds %d rows, want 3: %v", len(rep.Routes), rep.Routes)
	}
}

func TestTheReportNamesTheMiddlewareChain(t *testing.T) {
	rep, err := reportRouter(t).Report()
	if err != nil {
		t.Fatalf("Report returned an error: %v", err)
	}
	row, ok := rep.Route(http.MethodPost, "/api/things")
	if !ok {
		t.Fatal("the report holds no row for POST /api/things")
	}
	want := []string{"request_id", "recover", "csrf"}
	if len(row.Middleware) != len(want) {
		t.Fatalf("the chain is %v, want %v", row.Middleware, want)
	}
	for i := range want {
		if row.Middleware[i] != want[i] {
			t.Fatalf("the chain is %v, want %v", row.Middleware, want)
		}
	}
}

func TestTheReportDoesNotGiveAGroupMiddlewareToAnOutsideRoute(t *testing.T) {
	rep, err := reportRouter(t).Report()
	if err != nil {
		t.Fatalf("Report returned an error: %v", err)
	}
	row, _ := rep.Route(http.MethodGet, "/things")
	for _, name := range row.Middleware {
		if name == "csrf" {
			t.Fatalf("a route outside the group carries the group middleware: %v", row.Middleware)
		}
	}
}

func TestTheReportNamesTheHandler(t *testing.T) {
	r := router.New()
	r.Get("/things", listThings)
	rep, err := r.Report()
	if err != nil {
		t.Fatalf("Report returned an error: %v", err)
	}
	row, _ := rep.Route(http.MethodGet, "/things")
	if !strings.Contains(row.Handler, "listThings") {
		t.Fatalf("the handler is %q, want a name that holds listThings", row.Handler)
	}
}

func listThings(*router.Ctx) (router.Response, error) { return router.NoContent(), nil }

func TestTheReportNamesTheRegistrationSite(t *testing.T) {
	r := router.New()
	r.Get("/things", listThings)
	rep, err := r.Report()
	if err != nil {
		t.Fatalf("Report returned an error: %v", err)
	}
	row, _ := rep.Route(http.MethodGet, "/things")
	if !strings.HasSuffix(row.File, "routes_test.go") {
		t.Fatalf("the file is %q, want the test file", row.File)
	}
	if row.Line == 0 {
		t.Fatal("the row carries no line")
	}
}

func TestTheReportMarksAMountedHandler(t *testing.T) {
	rep, err := reportRouter(t).Report()
	if err != nil {
		t.Fatalf("Report returned an error: %v", err)
	}
	row, ok := rep.Route("*", "/admin/")
	if !ok {
		t.Fatalf("the report holds no row for the mount: %v", rep.Routes)
	}
	if !row.Mounted {
		t.Fatal("the mounted row is not marked")
	}
}

func TestTheReportPrintsOneLineForEachRoute(t *testing.T) {
	rep, err := reportRouter(t).Report()
	if err != nil {
		t.Fatalf("Report returned an error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(rep.String()), "\n")
	if len(lines) != len(rep.Routes)+1 {
		t.Fatalf("the report printed %d lines, want %d:\n%s", len(lines), len(rep.Routes)+1, rep.String())
	}
}

func TestTheReportIsStableAcrossRuns(t *testing.T) {
	first, err := json.Marshal(mustReport(t))
	if err != nil {
		t.Fatalf("Marshal returned an error: %v", err)
	}
	for range 5 {
		next, err := json.Marshal(mustReport(t))
		if err != nil {
			t.Fatalf("Marshal returned an error: %v", err)
		}
		if string(first) != string(next) {
			t.Fatalf("the JSON report changed between runs:\n%s\n%s", first, next)
		}
	}
}

func mustReport(t *testing.T) *router.Report {
	t.Helper()
	rep, err := reportRouter(t).Report()
	if err != nil {
		t.Fatalf("Report returned an error: %v", err)
	}
	return rep
}

func TestTheReportCarriesTheSchemaIdentifier(t *testing.T) {
	b, err := json.Marshal(mustReport(t))
	if err != nil {
		t.Fatalf("Marshal returned an error: %v", err)
	}
	var doc struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("Unmarshal returned an error: %v", err)
	}
	if doc.Schema != router.ReportSchemaID {
		t.Fatalf("schema = %q, want %q", doc.Schema, router.ReportSchemaID)
	}
}

func TestReportReturnsTheRegistrationFaults(t *testing.T) {
	r := router.New()
	r.Get("/things", ok("first"))
	r.Get("/things", ok("second"))
	if _, err := r.Report(); err == nil {
		t.Fatal("Report accepted two handlers on one pattern")
	}
}

// validate checks a document against the subset of JSON Schema that
// router/schema.json uses. A JSON Schema library is a dependency the SDD does
// not name, so the check lives here. See AGENTS.md.
func validate(t *testing.T, schema, doc any, path string) {
	t.Helper()
	s, ok := schema.(map[string]any)
	if !ok {
		t.Fatalf("%s: the schema node is not an object", path)
	}
	if want, ok := s["const"]; ok && fmt.Sprintf("%v", want) != fmt.Sprintf("%v", doc) {
		t.Fatalf("%s: value %v, want the constant %v", path, doc, want)
	}
	kind, _ := s["type"].(string)
	switch kind {
	case "object":
		obj, ok := doc.(map[string]any)
		if !ok {
			t.Fatalf("%s: value is %T, want an object", path, doc)
		}
		props, _ := s["properties"].(map[string]any)
		if raw, ok := s["required"].([]any); ok {
			for _, name := range raw {
				if _, ok := obj[name.(string)]; !ok {
					t.Fatalf("%s: the required member %q is absent", path, name)
				}
			}
		}
		if extra, ok := s["additionalProperties"].(bool); ok && !extra {
			for name := range obj {
				if _, ok := props[name]; !ok {
					t.Fatalf("%s: the member %q is not in the schema", path, name)
				}
			}
		}
		for name, sub := range props {
			if v, ok := obj[name]; ok {
				validate(t, sub, v, path+"/"+name)
			}
		}
	case "array":
		items, ok := doc.([]any)
		if !ok {
			t.Fatalf("%s: value is %T, want an array", path, doc)
		}
		if sub, ok := s["items"]; ok {
			for i, v := range items {
				validate(t, sub, v, fmt.Sprintf("%s/%d", path, i))
			}
		}
	case "string":
		if _, ok := doc.(string); !ok {
			t.Fatalf("%s: value is %T, want a string", path, doc)
		}
	case "boolean":
		if _, ok := doc.(bool); !ok {
			t.Fatalf("%s: value is %T, want a boolean", path, doc)
		}
	case "integer":
		n, ok := doc.(float64)
		if !ok || n != float64(int(n)) {
			t.Fatalf("%s: value is %v, want an integer", path, doc)
		}
	case "":
	default:
		t.Fatalf("%s: the test does not support the schema type %q", path, kind)
	}
}

func TestTheJSONReportValidatesAgainstTheSchema(t *testing.T) {
	var schema any
	if err := json.Unmarshal(router.ReportSchema, &schema); err != nil {
		t.Fatalf("router/schema.json does not parse: %v", err)
	}
	b, err := json.Marshal(mustReport(t))
	if err != nil {
		t.Fatalf("Marshal returned an error: %v", err)
	}
	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("Unmarshal returned an error: %v", err)
	}
	validate(t, schema, doc, "")
}

func TestAnEmptyReportValidatesAgainstTheSchema(t *testing.T) {
	var schema any
	if err := json.Unmarshal(router.ReportSchema, &schema); err != nil {
		t.Fatalf("router/schema.json does not parse: %v", err)
	}
	rep, err := router.New().Report()
	if err != nil {
		t.Fatalf("Report returned an error: %v", err)
	}
	b, _ := json.Marshal(rep)
	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("Unmarshal returned an error: %v", err)
	}
	validate(t, schema, doc, "")
}

func TestTheSchemaClosesItsObjects(t *testing.T) {
	if !strings.Contains(string(router.ReportSchema), `"additionalProperties": false`) {
		t.Fatal("router/schema.json does not close its objects")
	}
}
