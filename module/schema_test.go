package module_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/alternayte/avero/module"
)

// root holds the whole schema, so that a $ref resolves.
type root map[string]any

// validate checks a document against the subset of JSON Schema that
// module/schema.json uses. The subset holds type, properties, required,
// additionalProperties, items, enum, const and a local $ref. A JSON Schema
// library is a dependency that the SDD does not name, so the check lives here.
// See AGENTS.md.
func validate(t *testing.T, doc root, schema, value any, path string) {
	t.Helper()
	s, ok := schema.(map[string]any)
	if !ok {
		t.Fatalf("%s: the schema node is not an object", path)
	}
	if ref, ok := s["$ref"].(string); ok {
		validate(t, doc, resolve(t, doc, ref), value, path)
		return
	}
	if want, ok := s["const"]; ok && fmt.Sprintf("%v", want) != fmt.Sprintf("%v", value) {
		t.Fatalf("%s: value %v, want the constant %v", path, value, want)
	}
	if raw, ok := s["enum"].([]any); ok {
		found := false
		for _, want := range raw {
			if fmt.Sprintf("%v", want) == fmt.Sprintf("%v", value) {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: value %v is outside the enum %v", path, value, raw)
		}
	}
	kind, _ := s["type"].(string)
	switch kind {
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("%s: value is %T, want an object", path, value)
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
				validate(t, doc, sub, v, path+"/"+name)
			}
		}
	case "array":
		items, ok := value.([]any)
		if !ok {
			t.Fatalf("%s: value is %T, want an array", path, value)
		}
		if sub, ok := s["items"]; ok {
			for i, v := range items {
				validate(t, doc, sub, v, fmt.Sprintf("%s/%d", path, i))
			}
		}
	case "string":
		if _, ok := value.(string); !ok {
			t.Fatalf("%s: value is %T, want a string", path, value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			t.Fatalf("%s: value is %T, want a boolean", path, value)
		}
	case "":
	default:
		t.Fatalf("%s: the test does not support the schema type %q", path, kind)
	}
}

// resolve returns the node that a local $ref names.
func resolve(t *testing.T, doc root, ref string) any {
	t.Helper()
	var node any = map[string]any(doc)
	for _, step := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		obj, ok := node.(map[string]any)
		if !ok {
			t.Fatalf("the reference %q does not resolve", ref)
		}
		node = obj[step]
	}
	return node
}

// schemaAndDoc returns the schema and the JSON report of one set.
func schemaAndDoc(t *testing.T, set *module.Set) (root, any) {
	t.Helper()
	var schema root
	if err := json.Unmarshal(module.ReportSchema, &schema); err != nil {
		t.Fatalf("module/schema.json does not parse: %v", err)
	}
	rep, err := set.Report()
	if err != nil {
		t.Fatalf("Report returned %v, want nil", err)
	}
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("Marshal returned an error: %v", err)
	}
	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("Unmarshal returned an error: %v", err)
	}
	return schema, doc
}

func TestTheJSONReportValidatesAgainstTheSchema(t *testing.T) {
	schema, doc := schemaAndDoc(t, module.Modules(full{}, bare{}))
	validate(t, schema, map[string]any(schema), doc, "")
}

func TestAnEmptySetValidatesAgainstTheSchema(t *testing.T) {
	schema, doc := schemaAndDoc(t, module.Modules())
	validate(t, schema, map[string]any(schema), doc, "")
}

func TestTheReportCarriesTheSchemaIdentifier(t *testing.T) {
	_, doc := schemaAndDoc(t, module.Modules(bare{}))
	obj := doc.(map[string]any)
	if obj["schema"] != module.ReportSchemaID {
		t.Fatalf("schema = %v, want %q", obj["schema"], module.ReportSchemaID)
	}
}

func TestTheSchemaRejectsAnUnknownMember(t *testing.T) {
	// The schema must close its objects. A test that passes against an open
	// schema proves nothing.
	if !strings.Contains(string(module.ReportSchema), `"additionalProperties": false`) {
		t.Fatal("module/schema.json does not close its objects")
	}
}

func TestWriteReportWritesTheTableAndTheJSON(t *testing.T) {
	set := module.Modules(full{})
	var text bytes.Buffer
	if err := set.WriteReport(&text, false); err != nil {
		t.Fatalf("WriteReport returned %v, want nil", err)
	}
	if !strings.Contains(text.String(), "billing") {
		t.Fatalf("the table does not name the module:\n%s", text.String())
	}
	var out bytes.Buffer
	if err := set.WriteReport(&out, true); err != nil {
		t.Fatalf("WriteReport returned %v, want nil", err)
	}
	if !strings.Contains(out.String(), module.ReportSchemaID) {
		t.Fatalf("the JSON does not carry the schema:\n%s", out.String())
	}
}

func TestWriteReportReturnsTheFault(t *testing.T) {
	var out bytes.Buffer
	if err := module.Modules(full{}, clash{}).WriteReport(&out, true); err == nil {
		t.Fatal("WriteReport returned nil, want the fault")
	}
	if out.Len() != 0 {
		t.Fatalf("WriteReport wrote %q, want nothing", out.String())
	}
}
