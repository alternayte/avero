package mcp_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/avero/codegen"
	"github.com/alternayte/avero/mcp"
)

// faultsOf runs explain_error and returns the faults.
func faultsOf(t *testing.T, dir, message string) []mcp.Fault {
	t.Helper()
	body, err := json.Marshal(mcp.Explain(dir, message))
	if err != nil {
		t.Fatalf("Marshal returned %v", err)
	}
	var doc struct {
		Faults []mcp.Fault `json:"faults"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("Unmarshal returned %v", err)
	}
	return doc.Faults
}

func TestExplainMapsACompileFaultOfGeneratedCodeToTheSourceOfThePerson(t *testing.T) {
	// The generator writes the binding of an input type. A fault in that file
	// must name the field of the person, never the generated line. See DX-6.
	dir := t.TempDir()
	source := `package posts

import "github.com/alternayte/avero/router"

type Module struct{}

// CreateInput holds the form of a new post.
type CreateInput struct {
	// Title is the name that a person reads.
	Title int ` + "`form:\"title\" validate:\"required\"`" + `
}

func (m *Module) Create(c *router.Ctx, in CreateInput) (router.Response, error) {
	return router.NoContent(), nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "input.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	if _, err := codegen.Generate(dir); err != nil {
		t.Fatalf("Generate returned %v", err)
	}

	// The line of the fault is the line of the generated Bind that reads the
	// field.
	generated, err := os.ReadFile(filepath.Join(dir, codegen.GeneratedFile))
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}
	line := 0
	for i, text := range strings.Split(string(generated), "\n") {
		if strings.Contains(text, "in.Title =") {
			line = i + 1
		}
	}
	if line == 0 {
		t.Fatalf("the generated file holds no assignment of the field:\n%s", generated)
	}

	message := codegen.GeneratedFile + ":" + itoa(line) + ":3: cannot use v (variable of type string) as int value in assignment to in.Title"
	faults := faultsOf(t, dir, message)
	if len(faults) != 1 {
		t.Fatalf("Explain returned %d faults, want 1", len(faults))
	}
	fault := faults[0]
	if !fault.Generated {
		t.Fatal("the fault does not report a generated file")
	}
	if filepath.Base(fault.SourceFile) != "input.go" {
		t.Fatalf("the fault names %q, want input.go", fault.SourceFile)
	}
	if fault.SourceLine != 10 {
		t.Fatalf("the fault names line %d, want the line of the field Title", fault.SourceLine)
	}
	if !strings.Contains(fault.Repair, "Title") || !strings.Contains(fault.Repair, "avero generate") {
		t.Fatalf("the repair is %q", fault.Repair)
	}
}

func TestExplainReadsAnAveroFaultWithItsRepair(t *testing.T) {
	message := "internal/features/posts/module.go:41:2: the GET pattern \"posts\" does not start with a slash\n  → Write the pattern as `/posts`\n"
	faults := faultsOf(t, t.TempDir(), message)
	if len(faults) != 1 {
		t.Fatalf("Explain returned %d faults, want 1", len(faults))
	}
	fault := faults[0]
	if fault.Kind != mcp.KindAvero {
		t.Fatalf("the kind is %q, want %q", fault.Kind, mcp.KindAvero)
	}
	if fault.File != "internal/features/posts/module.go" || fault.Line != 41 || fault.Column != 2 {
		t.Fatalf("the position is %s:%d:%d", fault.File, fault.Line, fault.Column)
	}
	if fault.Repair != "Write the pattern as `/posts`" {
		t.Fatalf("the repair is %q", fault.Repair)
	}
	if fault.Generated {
		t.Fatal("the fault reports a generated file, and the file is one of the person")
	}
}

func TestExplainReadsEveryFaultOfOneOutput(t *testing.T) {
	message := "# blog\n./main.go:12:5: undefined: wire\n./wire.go:8:2: \"embed\" imported and not used\n"
	faults := faultsOf(t, t.TempDir(), message)
	if len(faults) != 2 {
		t.Fatalf("Explain returned %d faults, want 2", len(faults))
	}
	if faults[0].Line != 12 || faults[1].Line != 8 {
		t.Fatalf("the faults are %+v", faults)
	}
	for _, fault := range faults {
		if fault.Repair == "" {
			t.Fatalf("the fault %+v states no repair", fault)
		}
	}
}

func TestExplainStatesTheRepairForATextWithNoPosition(t *testing.T) {
	faults := faultsOf(t, t.TempDir(), "it does not work")
	if len(faults) != 1 || faults[0].Kind != mcp.KindUnknown {
		t.Fatalf("Explain returned %+v", faults)
	}
	if faults[0].Repair == "" {
		t.Fatal("the fault states no repair")
	}
}

// itoa returns the decimal form of a line number.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

func TestExplainReadsAPositionAfterAPrefix(t *testing.T) {
	// `go vet` writes its position after the name of the step.
	faults := faultsOf(t, t.TempDir(), "vet: ./broken.go:3:28: cannot use \"x\" as int value")
	if len(faults) != 1 {
		t.Fatalf("Explain returned %d faults, want 1", len(faults))
	}
	if faults[0].File != "broken.go" || faults[0].Line != 3 || faults[0].Column != 28 {
		t.Fatalf("the position is %s:%d:%d", faults[0].File, faults[0].Line, faults[0].Column)
	}
}
