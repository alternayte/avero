package module_test

import (
	"slices"
	"testing"

	"github.com/alternayte/avero/module"
)

// modelOnly implements ModelModule and no DescribeModule, so the module
// system builds the whole description. See AN-3.
type modelOnly struct{}

func (modelOnly) Name() string { return "posts" }

func (modelOnly) Models() []module.ModelDesc {
	return []module.ModelDesc{{
		Name:   "Post",
		Table:  "posts",
		Fields: []module.FieldDesc{{Name: "Title", Type: "string"}},
	}}
}

// The description of a module carries the models that the generator wrote.
func TestTheDescriptionCarriesTheModels(t *testing.T) {
	set := module.Modules(modelOnly{})
	if err := set.Err(); err != nil {
		t.Fatalf("the inspection failed: %v", err)
	}
	rep, err := set.Report()
	if err != nil {
		t.Fatalf("the report failed: %v", err)
	}
	rows := rep.Modules
	if len(rows) != 1 {
		t.Fatalf("the set holds %d rows", len(rows))
	}
	models := rows[0].Description.Models
	if len(models) != 1 || models[0].Table != "posts" {
		t.Fatalf("the description holds the models %+v", models)
	}
	if !slices.Contains(rows[0].Interfaces, "ModelModule") {
		t.Fatalf("the row names the interfaces %v", rows[0].Interfaces)
	}
}
