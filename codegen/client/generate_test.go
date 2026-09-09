package client_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/avero/codegen/client"
)

// pkg writes one package with this body and returns its directory.
func pkg(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	source := "package api\n\nimport \"context\"\n\nvar _ = context.Background\n\n" + body
	if err := os.WriteFile(filepath.Join(dir, "api.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	return dir
}

// faultOf runs the generator and returns its first fault.
func faultOf(t *testing.T, body string) *client.GenFault {
	t.Helper()
	_, err := client.Generate(pkg(t, body))
	if err == nil {
		t.Fatal("Generate returned nil, want a fault")
	}
	var faults *client.GenFaults
	if !errors.As(err, &faults) {
		t.Fatalf("the error is %T, want *client.GenFaults: %v", err, err)
	}
	return faults.Faults[0]
}

func TestAMalformedDirectiveNamesTheFileTheLineAndTheRepair(t *testing.T) {
	f := faultOf(t, `//avero:client base=https://api.example.com
type Example interface {
	//avero:GET /things/{id}
	GetThing(ctx context.Context, id string) (Thing, error)
}

type Thing struct{}
`)
	if !strings.HasSuffix(f.File, "api.go") {
		t.Fatalf("the fault names %q, want the file of the person", f.File)
	}
	if f.Line != 7 || f.Column == 0 {
		t.Fatalf("the fault names line %d and column %d", f.Line, f.Column)
	}
	if !strings.Contains(f.Message, "quotation marks") {
		t.Fatalf("the message is %q", f.Message)
	}
	if !strings.Contains(f.Repair, `key="value"`) {
		t.Fatalf("the repair is %q", f.Repair)
	}
	if !strings.Contains(f.Error(), "→") {
		t.Fatalf("the fault prints %q", f.Error())
	}
}

func TestAnAbsentBaseIsAFault(t *testing.T) {
	f := faultOf(t, "//avero:client auth=\"bearer\"\ntype Example interface {\n\t//avero:GET /things\n\tList(ctx context.Context) error\n}\n")
	if !strings.Contains(f.Message, "base address") || !strings.Contains(f.Repair, "base=") {
		t.Fatalf("the fault is %q and %q", f.Message, f.Repair)
	}
}

func TestAnUnknownHTTPMethodIsAFault(t *testing.T) {
	f := faultOf(t, "//avero:client base=\"https://api.example.com\"\ntype Example interface {\n\t//avero:FETCH /things\n\tList(ctx context.Context) error\n}\n")
	if !strings.Contains(f.Message, "FETCH") {
		t.Fatalf("the message is %q", f.Message)
	}
}

func TestAPathWithNoSlashIsAFault(t *testing.T) {
	f := faultOf(t, "//avero:client base=\"https://api.example.com\"\ntype Example interface {\n\t//avero:GET things\n\tList(ctx context.Context) error\n}\n")
	if !strings.Contains(f.Repair, "`/things`") {
		t.Fatalf("the repair is %q", f.Repair)
	}
}

func TestAPlaceholderWithNoParameterIsAFault(t *testing.T) {
	f := faultOf(t, "//avero:client base=\"https://api.example.com\"\ntype Example interface {\n\t//avero:GET /things/{id}\n\tList(ctx context.Context) error\n}\n")
	if !strings.Contains(f.Message, "{id}") {
		t.Fatalf("the message is %q", f.Message)
	}
}

func TestAMissingContextIsAFault(t *testing.T) {
	f := faultOf(t, "//avero:client base=\"https://api.example.com\"\ntype Example interface {\n\t//avero:GET /things\n\tList(id string) error\n}\n")
	if !strings.Contains(f.Message, "context.Context") {
		t.Fatalf("the message is %q", f.Message)
	}
}

func TestAMethodWithNoDirectiveIsAFault(t *testing.T) {
	f := faultOf(t, "//avero:client base=\"https://api.example.com\"\ntype Example interface {\n\tList(ctx context.Context) error\n}\n")
	if !strings.Contains(f.Message, "carries no directive") {
		t.Fatalf("the message is %q", f.Message)
	}
}

func TestAGetWithABodyIsAFault(t *testing.T) {
	f := faultOf(t, "//avero:client base=\"https://api.example.com\"\ntype Example interface {\n\t//avero:GET /things\n\tList(ctx context.Context, body NewThing) error\n}\n\ntype NewThing struct{ Title string `json:\"title\"` }\n")
	if !strings.Contains(f.Message, "carries a body") {
		t.Fatalf("the message is %q", f.Message)
	}
}

func TestAWrongResultShapeIsAFault(t *testing.T) {
	f := faultOf(t, "//avero:client base=\"https://api.example.com\"\ntype Example interface {\n\t//avero:GET /things\n\tList(ctx context.Context) (int, int, error)\n}\n")
	if !strings.Contains(f.Repair, "(T, error)") {
		t.Fatalf("the repair is %q", f.Repair)
	}
}

func TestAnUnknownOptionIsAFault(t *testing.T) {
	f := faultOf(t, "//avero:client base=\"https://api.example.com\" retries=\"3\"\ntype Example interface {\n\t//avero:GET /things\n\tList(ctx context.Context) error\n}\n")
	if !strings.Contains(f.Message, "retries") || !strings.Contains(f.Repair, "attempts") {
		t.Fatalf("the fault is %q and %q", f.Message, f.Repair)
	}
}

func TestATimeoutThatIsNoDurationIsAFault(t *testing.T) {
	f := faultOf(t, "//avero:client base=\"https://api.example.com\" timeout=\"5\"\ntype Example interface {\n\t//avero:GET /things\n\tList(ctx context.Context) error\n}\n")
	if !strings.Contains(f.Message, "duration") {
		t.Fatalf("the message is %q", f.Message)
	}
}

func TestGenerateWritesNoFileForAPackageWithNoClient(t *testing.T) {
	written, err := client.Generate(pkg(t, "type Thing struct{}\n"))
	if err != nil {
		t.Fatalf("Generate returned %v, want nil", err)
	}
	if len(written) != 0 {
		t.Fatalf("Generate wrote %v", written)
	}
}

func TestGenerateIsStableAndCheckAgrees(t *testing.T) {
	dir := pkg(t, "//avero:client base=\"https://api.example.com\"\ntype Example interface {\n\t//avero:GET /things/{id}\n\tGetThing(ctx context.Context, id string) (Thing, error)\n}\n\ntype Thing struct{ ID string `json:\"id\"` }\n")
	first, err := client.Generate(dir)
	if err != nil {
		t.Fatalf("Generate returned %v, want nil", err)
	}
	body, err := os.ReadFile(first[0])
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}
	if _, err := client.Generate(dir); err != nil {
		t.Fatalf("Generate returned %v, want nil", err)
	}
	again, err := os.ReadFile(first[0])
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}
	if string(body) != string(again) {
		t.Fatal("two runs of the generator wrote two files")
	}
	if err := client.Check(dir); err != nil {
		t.Fatalf("Check returned %v, want nil", err)
	}
}

func TestCheckReportsAStaleFile(t *testing.T) {
	dir := pkg(t, "//avero:client base=\"https://api.example.com\"\ntype Example interface {\n\t//avero:GET /things\n\tList(ctx context.Context) error\n}\n")
	if _, err := client.Generate(dir); err != nil {
		t.Fatalf("Generate returned %v, want nil", err)
	}
	name := filepath.Join(dir, client.GeneratedFile)
	if err := os.WriteFile(name, []byte("package api\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	err := client.Check(dir)
	var stale *client.StaleFault
	if !errors.As(err, &stale) {
		t.Fatalf("Check returned %T, want *client.StaleFault", err)
	}
	if !strings.Contains(stale.Error(), "avero generate") {
		t.Fatalf("the fault prints %q", stale.Error())
	}
}

func TestCheckReportsAnAbsentFile(t *testing.T) {
	dir := pkg(t, "//avero:client base=\"https://api.example.com\"\ntype Example interface {\n\t//avero:GET /things\n\tList(ctx context.Context) error\n}\n")
	if err := client.Check(dir); err == nil {
		t.Fatal("Check returned nil, want a fault")
	}
}
