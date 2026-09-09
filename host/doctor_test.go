package host_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/alternayte/avero/config"
	"github.com/alternayte/avero/host"
)

// failing returns a check that does not hold.
func failing(name, message, repair string) host.Check {
	return host.Check{
		Name:   name,
		Repair: repair,
		Run:    func(context.Context) error { return errors.New(message) },
	}
}

// passing returns a check that holds.
func passing(name string) host.Check {
	return host.Check{Name: name, Repair: "-", Run: func(context.Context) error { return nil }}
}

func TestTheDoctorReportsEveryFaultWithItsRepair(t *testing.T) {
	// A configuration with no AVERO_SECRET fails, and the report names the
	// variable and the repair. See DX-7 and DX-8.
	_, report, err := config.LoadFrom[config.BaseConfig](context.Background(), config.Loader{
		Lookup: func(string) (string, bool) { return "", false },
	})
	if err == nil {
		t.Fatal("the loader accepted a configuration with no secret")
	}
	rep := host.Doctor(context.Background(), report, err, []host.Check{
		passing("the database"),
		failing("the pending migrations", "2 migrations are pending", "Run `avero migrate up`"),
		failing("the broker", "the broker at localhost:5672 does not answer", "Start the broker"),
	})
	if rep.OK {
		t.Fatal("OK reads true, want false")
	}

	names := map[string]host.CheckResult{}
	for _, row := range rep.Checks {
		names[row.Name] = row
	}
	if row, ok := names["the variable AVERO_SECRET"]; !ok || row.State != host.StateFail || row.Repair == "" {
		t.Fatalf("the report holds %+v for the absent variable", row)
	}
	if row := names["the pending migrations"]; row.State != host.StateFail || !strings.Contains(row.Repair, "avero migrate up") {
		t.Fatalf("the report holds %+v for the pending migrations", row)
	}
	if row := names["the broker"]; row.State != host.StateFail || row.Repair == "" {
		t.Fatalf("the report holds %+v for the broker", row)
	}
	if row := names["the database"]; row.State != host.StatePass {
		t.Fatalf("the report holds %+v for the database", row)
	}
	table := rep.String()
	for _, want := range []string{"the broker", "→"} {
		if !strings.Contains(table, want) {
			t.Fatalf("the table does not hold %q:\n%s", want, table)
		}
	}
}

func TestTheDoctorReportsAPassingRun(t *testing.T) {
	rep := host.Doctor(context.Background(), nil, nil, []host.Check{passing("the database")})
	if !rep.OK {
		t.Fatalf("OK reads false: %+v", rep.Checks)
	}
	if !strings.Contains(rep.String(), "every check holds") {
		t.Fatalf("the table holds %q", rep.String())
	}
}

func TestTheDoctorReportValidatesAgainstItsSchema(t *testing.T) {
	var schema any
	if err := json.Unmarshal(host.DoctorSchema, &schema); err != nil {
		t.Fatalf("host/doctor_schema.json does not parse: %v", err)
	}
	rep := host.Doctor(context.Background(), nil, nil, []host.Check{
		passing("the database"),
		failing("the broker", "no answer", "Start the broker"),
	})
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("Marshal returned an error: %v", err)
	}
	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("Unmarshal returned an error: %v", err)
	}
	validateDoctor(t, schema, doc, "")
	if !strings.Contains(string(b), host.DoctorSchemaID) {
		t.Fatalf("the report carries no schema: %s", b)
	}
}

// validateDoctor checks a document against the subset of JSON Schema that
// host/doctor_schema.json uses. A JSON Schema library is a dependency that the
// SDD does not name, so the check lives here. See AGENTS.md.
func validateDoctor(t *testing.T, schema, doc any, path string) {
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
				validateDoctor(t, sub, v, path+"/"+name)
			}
		}
	case "array":
		items, ok := doc.([]any)
		if !ok {
			t.Fatalf("%s: value is %T, want an array", path, doc)
		}
		if sub, ok := s["items"]; ok {
			for i, v := range items {
				validateDoctor(t, sub, v, fmt.Sprintf("%s/%d", path, i))
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
