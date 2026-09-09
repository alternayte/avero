package verify_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/avero/verify"
)

// module writes a small Go module and returns its directory.
func module(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "go.mod", "module gate\n\ngo 1.26.2\n")
	write(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	for name, body := range files {
		write(t, dir, name, body)
	}
	return dir
}

// write puts one file into a directory.
func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
}

// steps returns the gate with a generate check that holds.
func steps() []verify.Step {
	return verify.Steps(func(string) error { return nil })
}

func TestTheGatePassesAndTheOrderIsDeterministic(t *testing.T) {
	dir := module(t, nil)
	first := verify.Run(context.Background(), dir, steps())
	second := verify.Run(context.Background(), dir, steps())

	if !first.OK || !second.OK {
		t.Fatalf("the gate failed: %s", first.String())
	}
	want := []string{"gofmt", "vet", "generate", "test", "build"}
	for _, rep := range []*verify.Report{first, second} {
		names := make([]string, 0, len(rep.Records))
		for _, row := range rep.Records {
			names = append(names, row.Name)
			if row.Status != verify.StatusPass {
				t.Fatalf("the step %s failed: %s", row.Name, row.Message)
			}
		}
		if strings.Join(names, ",") != strings.Join(want, ",") {
			t.Fatalf("the order is %v, want %v", names, want)
		}
	}
}

func TestAFailedStepCarriesTheFileTheLineAndTheMessage(t *testing.T) {
	dir := module(t, map[string]string{
		"broken.go": "package main\n\nfunc broken() int { return \"x\" }\n",
	})
	rep := verify.Run(context.Background(), dir, steps())
	if rep.OK {
		t.Fatal("the gate passed with a broken package")
	}
	found := false
	for _, row := range rep.Records {
		if row.Status != verify.StatusFail {
			continue
		}
		found = true
		if !strings.HasSuffix(row.File, "broken.go") {
			t.Fatalf("the record names %q", row.File)
		}
		if row.Line != 3 {
			t.Fatalf("the record names line %d, want 3", row.Line)
		}
		if row.Message == "" {
			t.Fatal("the record states no message")
		}
	}
	if !found {
		t.Fatalf("no record failed:\n%s", rep.String())
	}
}

func TestAFileThatNeedsAFormatFailsTheGate(t *testing.T) {
	// gofmt lists the file and exits zero, so the step reads the output.
	dir := module(t, map[string]string{"ugly.go": "package main\nfunc ugly()  {  }\n"})
	rep := verify.Run(context.Background(), dir, steps())
	if rep.OK {
		t.Fatal("the gate passed with a file that needs a format")
	}
	if rep.Records[0].Name != "gofmt" || rep.Records[0].Status != verify.StatusFail {
		t.Fatalf("the first record is %+v", rep.Records[0])
	}
}

func TestTheGenerateStepReportsItsFault(t *testing.T) {
	dir := module(t, nil)
	rep := verify.Run(context.Background(), dir, verify.Steps(func(string) error {
		return errors.New("zz_generated.go:4:2: the file is not current\n  → Run `avero generate`")
	}))
	if rep.OK {
		t.Fatal("the gate passed with a stale generated file")
	}
	for _, row := range rep.Records {
		if row.Name != "generate" {
			continue
		}
		if row.File != "zz_generated.go" || row.Line != 4 || row.Column != 2 {
			t.Fatalf("the record is %+v", row)
		}
	}
}

func TestEveryRecordCarriesItsDuration(t *testing.T) {
	rep := verify.Run(context.Background(), module(t, nil), steps())
	for _, row := range rep.Records {
		if row.DurationMS < 0 {
			t.Fatalf("the record %s holds %d milliseconds", row.Name, row.DurationMS)
		}
	}
}

func TestTheReportValidatesAgainstTheSchema(t *testing.T) {
	var schema any
	if err := json.Unmarshal(verify.Schema, &schema); err != nil {
		t.Fatalf("verify/schema.json does not parse: %v", err)
	}
	rep := verify.Run(context.Background(), module(t, map[string]string{
		"broken.go": "package main\n\nfunc broken() int { return \"x\" }\n",
	}), steps())
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("Marshal returned %v", err)
	}
	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("Unmarshal returned %v", err)
	}
	validate(t, schema, doc, "")
	if !strings.Contains(string(b), verify.SchemaID) {
		t.Fatalf("the report carries no schema: %s", b)
	}
}

// validate checks a document against the subset of JSON Schema that
// verify/schema.json uses. A JSON Schema library is a dependency that the SDD
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
