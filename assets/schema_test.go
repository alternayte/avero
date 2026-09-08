package assets_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/alternayte/avero/assets"
)

// validate checks a document against the subset of JSON Schema that
// assets/schema.json uses. The subset holds type, properties, required,
// additionalProperties as a schema or a boolean, items and const. A JSON
// Schema library is a dependency that the SDD does not name, so the check
// lives here. See AGENTS.md.
func validate(t *testing.T, schema, doc any, path string) {
	t.Helper()
	s, ok := schema.(map[string]any)
	if !ok {
		t.Fatalf("%s: the schema node is not an object", path)
	}
	if want, ok := s["const"]; ok && fmt.Sprintf("%v", want) != fmt.Sprintf("%v", doc) {
		t.Fatalf("%s: value %v, want the constant %v", path, doc, want)
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
		extra := s["additionalProperties"]
		if closed, ok := extra.(bool); ok && !closed {
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
		if sub, ok := extra.(map[string]any); ok {
			for name, v := range obj {
				if _, known := props[name]; !known {
					validate(t, sub, v, path+"/"+name)
				}
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

func TestTheManifestValidatesAgainstTheSchema(t *testing.T) {
	var schema any
	if err := json.Unmarshal(assets.ManifestSchema, &schema); err != nil {
		t.Fatalf("assets/schema.json does not parse: %v", err)
	}
	b, err := json.Marshal(manifest())
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
	if !strings.Contains(string(assets.ManifestSchema), `"additionalProperties": false`) {
		t.Fatal("assets/schema.json does not close its objects")
	}
}
