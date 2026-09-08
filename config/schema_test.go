package config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/alternayte/avero/config"
)

// validate checks a document against the subset of JSON Schema that
// config/schema.json uses. The subset holds type, properties, required,
// additionalProperties, items, enum and const. A dependency is not available,
// so the check lives here. See AGENTS.md, "add no dependency the SDD does not
// name".
func validate(t *testing.T, schema, doc any, path string) {
	t.Helper()
	s, ok := schema.(map[string]any)
	if !ok {
		t.Fatalf("%s: the schema node is not an object", path)
	}
	if want, ok := s["const"]; ok && !equal(want, doc) {
		t.Fatalf("%s: value %v, want the constant %v", path, doc, want)
	}
	if raw, ok := s["enum"]; ok {
		found := false
		for _, want := range raw.([]any) {
			if equal(want, doc) {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: value %v is outside the enum %v", path, doc, raw)
		}
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
	case "":
	default:
		t.Fatalf("%s: the test does not support the schema type %q", path, kind)
	}
}

func equal(a, b any) bool { return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b) }

func TestReportJSONValidatesAgainstTheSchema(t *testing.T) {
	var schema any
	if err := json.Unmarshal(config.ReportSchema, &schema); err != nil {
		t.Fatalf("config/schema.json does not parse: %v", err)
	}
	b, err := json.Marshal(report(t))
	if err != nil {
		t.Fatalf("Marshal returned an error: %v", err)
	}
	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("Unmarshal returned an error: %v", err)
	}
	validate(t, schema, doc, "")
}

func TestABaseConfigReportValidatesAgainstTheSchema(t *testing.T) {
	var schema any
	if err := json.Unmarshal(config.ReportSchema, &schema); err != nil {
		t.Fatalf("config/schema.json does not parse: %v", err)
	}
	_, rep, err := config.LoadFrom[config.BaseConfig](context.Background(), env(nil))
	if err != nil {
		t.Fatalf("LoadFrom returned an error: %v", err)
	}
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("Marshal returned an error: %v", err)
	}
	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("Unmarshal returned an error: %v", err)
	}
	validate(t, schema, doc, "")
}

func TestTheSchemaRejectsAnUnknownMember(t *testing.T) {
	// The schema must close its objects. A test that passes against an open
	// schema proves nothing.
	if !strings.Contains(string(config.ReportSchema), `"additionalProperties": false`) {
		t.Fatal("config/schema.json does not close its objects")
	}
}
