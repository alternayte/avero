package codegen_test

import (
	"strings"
	"testing"
)

// The fault of a tag must name the file, the line and the column of the struct
// field that the person wrote. See DX-6.

func TestAnUnknownRuleIsAFaultAtTheField(t *testing.T) {
	f := faultOf(t, `type CreateInput struct {
	Title string `+"`json:\"title\" validate:\"requird\"`"+`
}

func (m *Module) Create(ctx *router.Ctx, in CreateInput) (router.Response, error) {
	return router.NoContent(), nil
}
`)
	if !strings.HasSuffix(f.File, "posts.go") {
		t.Fatalf("the fault names the file %q", f.File)
	}
	if f.Line != 8 {
		t.Fatalf("the fault names line %d, want 8", f.Line)
	}
	if f.Column == 0 {
		t.Fatal("the fault names no column")
	}
	if !strings.Contains(f.Message, "requird") {
		t.Fatalf("the message does not carry the rule: %q", f.Message)
	}
	if f.Repair == "" {
		t.Fatal("the fault states no repair")
	}
	if !strings.Contains(f.Error(), "posts.go:8:") {
		t.Fatalf("the error does not print the position: %q", f.Error())
	}
}

func TestTheColumnPointsAtTheField(t *testing.T) {
	f := faultOf(t, `type CreateInput struct {
	Title string `+"`json:\"title\" validate:\"requird\"`"+`
}

func (m *Module) Create(ctx *router.Ctx, in CreateInput) (router.Response, error) {
	return router.NoContent(), nil
}
`)
	// The field is indented by one tab, so it starts at column 2.
	if f.Column != 2 {
		t.Fatalf("the fault names column %d, want 2", f.Column)
	}
}

func TestAFaultNeverPointsIntoTheGenerator(t *testing.T) {
	f := faultOf(t, `type CreateInput struct {
	Title string `+"`json:\"title\" validate:\"requird\"`"+`
}

func (m *Module) Create(ctx *router.Ctx, in CreateInput) (router.Response, error) {
	return router.NoContent(), nil
}
`)
	for _, bad := range []string{"codegen", "zz_generated", "avero/router"} {
		if strings.Contains(f.File, bad) {
			t.Fatalf("the fault points into %s: %q", bad, f.File)
		}
	}
}

func TestAMinWithNoNumberIsAFault(t *testing.T) {
	f := faultOf(t, `type CreateInput struct {
	Title string `+"`json:\"title\" validate:\"min=lots\"`"+`
}

func (m *Module) Create(ctx *router.Ctx, in CreateInput) (router.Response, error) {
	return router.NoContent(), nil
}
`)
	if !strings.Contains(f.Message, "lots") || f.Line != 8 {
		t.Fatalf("the fault is %q at line %d", f.Message, f.Line)
	}
}

func TestOneofWithNoValueIsAFault(t *testing.T) {
	f := faultOf(t, `type CreateInput struct {
	Role string `+"`json:\"role\" validate:\"oneof=\"`"+`
}

func (m *Module) Create(ctx *router.Ctx, in CreateInput) (router.Response, error) {
	return router.NoContent(), nil
}
`)
	if !strings.Contains(f.Message, "oneof") {
		t.Fatalf("the message is %q", f.Message)
	}
}

func TestRequiredOnABoolIsAFault(t *testing.T) {
	// A false boolean is a value that a person meant to send.
	f := faultOf(t, `type CreateInput struct {
	Notify bool `+"`json:\"notify\" validate:\"required\"`"+`
}

func (m *Module) Create(ctx *router.Ctx, in CreateInput) (router.Response, error) {
	return router.NoContent(), nil
}
`)
	if !strings.Contains(f.Message, "bool") {
		t.Fatalf("the message is %q", f.Message)
	}
}

func TestEmailOnANumberIsAFault(t *testing.T) {
	f := faultOf(t, `type CreateInput struct {
	Count int `+"`json:\"count\" validate:\"email\"`"+`
}

func (m *Module) Create(ctx *router.Ctx, in CreateInput) (router.Response, error) {
	return router.NoContent(), nil
}
`)
	if !strings.Contains(f.Message, "email") {
		t.Fatalf("the message is %q", f.Message)
	}
}

func TestAnUnsupportedFieldTypeIsAFault(t *testing.T) {
	f := faultOf(t, `type CreateInput struct {
	Ratio complex128 `+"`json:\"ratio\"`"+`
}

func (m *Module) Create(ctx *router.Ctx, in CreateInput) (router.Response, error) {
	return router.NoContent(), nil
}
`)
	if !strings.Contains(f.Message, "complex128") {
		t.Fatalf("the message is %q", f.Message)
	}
}

func TestAFieldWithNoBindingTagIsAFault(t *testing.T) {
	// A field that binds from nothing is a silent fault at request time.
	f := faultOf(t, `type CreateInput struct {
	Title string
}

func (m *Module) Create(ctx *router.Ctx, in CreateInput) (router.Response, error) {
	return router.NoContent(), nil
}
`)
	if !strings.Contains(f.Message, "Title") {
		t.Fatalf("the message is %q", f.Message)
	}
	if !strings.Contains(f.Repair, "-") {
		t.Fatalf("the repair does not name the way to skip the field: %q", f.Repair)
	}
}

func TestASkippedFieldIsNotAFault(t *testing.T) {
	out := generate(t, `type CreateInput struct {
	Title    string `+"`json:\"title\"`"+`
	Computed string `+"`avero:\"-\"`"+`
}

func (m *Module) Create(ctx *router.Ctx, in CreateInput) (router.Response, error) {
	return router.NoContent(), nil
}
`)
	if strings.Contains(out, "Computed") {
		t.Fatalf("the generator bound a skipped field:\n%s", out)
	}
}

func TestEveryFaultIsReported(t *testing.T) {
	dir := write(t, header+`type CreateInput struct {
	Title string `+"`json:\"title\" validate:\"requird\"`"+`
	Role  string `+"`json:\"role\" validate:\"oneof=\"`"+`
}

func (m *Module) Create(ctx *router.Ctx, in CreateInput) (router.Response, error) {
	return router.NoContent(), nil
}
`)
	_, err := generateFaults(dir)
	if err == nil {
		t.Fatal("Generate accepted two faults")
	}
	if n := strings.Count(err.Error(), "→"); n != 2 {
		t.Fatalf("the report holds %d repairs, want 2:\n%s", n, err.Error())
	}
}
