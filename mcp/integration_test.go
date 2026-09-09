//go:build integration

package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/avero/internal/cli"
	"github.com/alternayte/avero/mcp"
)

// The integration suite starts the server against a scaffolded application. It
// asserts the schema of each tool result, and it drives scaffold_slice and
// then run_verify. See the SDD, S16.

// application scaffolds an application and returns its directory.
func application(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("Abs returned %v", err)
	}
	dir := t.TempDir()
	code := cli.Run(context.Background(), cli.Streams{Out: os.Stderr, Err: os.Stderr, Dir: dir},
		[]string{"new", "blog", "--replace", root})
	if code != 0 {
		t.Fatal("avero new failed")
	}
	app := filepath.Join(dir, "blog")
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = app
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, out)
	}
	return app
}

// server returns a server wired as `avero mcp` wires it.
func server(t *testing.T, dir string) *mcp.Server {
	t.Helper()
	return &mcp.Server{Dir: dir, Slice: cli.Slice, Generate: cli.GenerateCheck}
}

// definition returns the schema of one tool result.
func definition(t *testing.T, tool string) any {
	t.Helper()
	var schema map[string]any
	if err := json.Unmarshal(mcp.Schema, &schema); err != nil {
		t.Fatalf("mcp/schema.json does not parse: %v", err)
	}
	defs, _ := schema["$defs"].(map[string]any)
	sub, ok := defs[tool]
	if !ok {
		t.Fatalf("mcp/schema.json holds no definition of %s", tool)
	}
	return sub
}

func TestEveryToolResultFollowsItsSchema(t *testing.T) {
	app := application(t)
	s := newSession(t, server(t, app))

	for _, tc := range []struct {
		tool string
		args map[string]any
		want string
	}{
		{tool: "list_modules", want: "posts"},
		{tool: "list_routes", want: "/posts/new"},
		{tool: "describe_module", args: map[string]any{"name": "posts"}, want: "posts"},
		{tool: "describe_model", args: map[string]any{"name": "Post"}, want: "posts"},
		{tool: "explain_error", args: map[string]any{"message": "./main.go:12:5: undefined: wire"}, want: "main.go"},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			result, ok := s.call(tc.tool, tc.args)
			if !ok {
				t.Fatalf("the tool failed: %v", result)
			}
			body, err := json.Marshal(result)
			if err != nil {
				t.Fatalf("Marshal returned %v", err)
			}
			if !strings.Contains(string(body), tc.want) {
				t.Fatalf("the result holds no %q:\n%s", tc.want, body)
			}
			var doc any
			if err := json.Unmarshal(body, &doc); err != nil {
				t.Fatalf("Unmarshal returned %v", err)
			}
			validate(t, definition(t, tc.tool), doc, "")
		})
	}
}

func TestScaffoldSliceThenRunVerifyPassesTheGate(t *testing.T) {
	app := application(t)
	s := newSession(t, server(t, app))

	written, ok := s.call("scaffold_slice", map[string]any{"name": "comment"})
	if !ok {
		t.Fatalf("scaffold_slice failed: %v", written)
	}
	validate(t, definition(t, "scaffold_slice"), decode(t, written), "")
	if written["registered"] != true {
		t.Fatalf("the slice did not register itself: %v", written)
	}
	if _, err := os.Stat(filepath.Join(app, "internal", "features", "comments", "module.go")); err != nil {
		t.Fatalf("the tool wrote no module: %v", err)
	}

	report, ok := s.call("run_verify", nil)
	if !ok {
		t.Fatalf("run_verify failed: %v", report)
	}
	validate(t, definition(t, "run_verify"), decode(t, report), "")
	if report["ok"] != true {
		body, _ := json.MarshalIndent(report, "", "  ")
		t.Fatalf("the gate failed after the slice:\n%s", body)
	}

	// The order of the records never changes.
	again, _ := s.call("run_verify", nil)
	if names(t, report) != names(t, again) {
		t.Fatalf("two runs gave %q and %q", names(t, report), names(t, again))
	}
}

func TestRunVerifyReportsAFailureWithItsPosition(t *testing.T) {
	app := application(t)
	broken := "package main\n\nfunc broken() int { return \"x\" }\n"
	if err := os.WriteFile(filepath.Join(app, "broken.go"), []byte(broken), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	s := newSession(t, server(t, app))
	report, ok := s.call("run_verify", nil)
	if !ok {
		t.Fatalf("run_verify failed: %v", report)
	}
	if report["ok"] != false {
		t.Fatalf("the gate passed with a broken package: %v", report)
	}
	records, _ := report["records"].([]any)
	found := false
	for _, raw := range records {
		row, _ := raw.(map[string]any)
		if row["status"] != "fail" {
			continue
		}
		found = true
		if !strings.Contains(fmt.Sprint(row["file"]), "broken.go") || row["line"] == float64(0) {
			t.Fatalf("the record is %v", row)
		}
		if fmt.Sprint(row["message"]) == "" {
			t.Fatalf("the record states no message: %v", row)
		}
	}
	if !found {
		t.Fatalf("no record failed: %v", report)
	}
}

// decode turns a result into a plain document.
func decode(t *testing.T, result map[string]any) any {
	t.Helper()
	body, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal returned %v", err)
	}
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("Unmarshal returned %v", err)
	}
	return doc
}

// names returns the order of the records of a verify report.
func names(t *testing.T, report map[string]any) string {
	t.Helper()
	records, _ := report["records"].([]any)
	out := make([]string, 0, len(records))
	for _, raw := range records {
		row, _ := raw.(map[string]any)
		out = append(out, fmt.Sprint(row["name"]))
	}
	return strings.Join(out, ",")
}

// validate checks a document against the subset of JSON Schema that
// mcp/schema.json uses. A JSON Schema library is a dependency that the SDD
// does not name, so the check lives here. See AGENTS.md.
func validate(t *testing.T, schema, doc any, path string) {
	t.Helper()
	s, ok := schema.(map[string]any)
	if !ok {
		t.Fatalf("%s: the schema node is not an object", path)
	}
	if want, ok := s["const"]; ok && fmt.Sprintf("%v", want) != fmt.Sprintf("%v", doc) {
		t.Fatalf("%s: value %v, want %v", path, doc, want)
	}
	if raw, ok := s["enum"].([]any); ok {
		found := false
		for _, want := range raw {
			if fmt.Sprintf("%v", want) == fmt.Sprintf("%v", doc) {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: value %v is outside the enum %v", path, doc, raw)
		}
	}
	switch kind, _ := s["type"].(string); kind {
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
		if closed, ok := s["additionalProperties"].(bool); ok && !closed {
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
