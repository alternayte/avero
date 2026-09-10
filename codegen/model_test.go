package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The generator reads the model package of a feature slice and writes the
// Models method of its module. See AN-3.
func TestTheGeneratorWritesTheModels(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "handlers.go"), `package posts

import "github.com/alternayte/avero/router"

type Module struct{}

// Show renders one post.
func (m *Module) Show(c *router.Ctx, in ShowInput) (router.Response, error) {
	return nil, nil
}
`)
	write(t, filepath.Join(dir, "input.go"), `package posts

type ShowInput struct {
	ID string `+"`path:\"id\"`"+`
}
`)
	modelDir := filepath.Join(dir, "model")
	if err := os.MkdirAll(modelDir, 0o750); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(modelDir, "post.go"), `package model

import (
	"github.com/alternayte/drel"
	"github.com/google/uuid"
)

type Post struct {
	drel.Model[uuid.UUID]

	Title string `+"`db:\"title\"`"+`
	Body  string `+"`db:\"body\"`"+`
}
`)
	write(t, filepath.Join(modelDir, "post_drel.go"), `package model

import "github.com/alternayte/drel"

var PostMeta = drel.ModelMeta[Post]{
	Table: "posts",
}
`)

	if _, err := Generate(dir); err != nil {
		t.Fatalf("the generator failed: %v", err)
	}
	source := read(t, filepath.Join(dir, GeneratedFile))

	for _, want := range []string{
		"func (m *Module) Models() []module.ModelDesc {",
		`Name:  "Post"`,
		`Table: "posts"`,
		`{Name: "ID", Type: "uuid.UUID"}`,
		`{Name: "Title", Type: "string"}`,
		`{Name: "Body", Type: "string"}`,
		`{Name: "CreatedAt", Type: "time.Time"}`,
		`{Name: "UpdatedAt", Type: "time.Time"}`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("the generated file holds no %q\n%s", want, source)
		}
	}
}

// A feature slice with no model package writes no Models method.
func TestTheGeneratorWritesNoModelsWithoutAModelPackage(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "handlers.go"), `package posts

import "github.com/alternayte/avero/router"

type Module struct{}

// Show renders one post.
func (m *Module) Show(c *router.Ctx, in ShowInput) (router.Response, error) {
	return nil, nil
}
`)
	write(t, filepath.Join(dir, "input.go"), `package posts

type ShowInput struct {
	ID string `+"`path:\"id\"`"+`
}
`)
	if _, err := Generate(dir); err != nil {
		t.Fatalf("the generator failed: %v", err)
	}
	if source := read(t, filepath.Join(dir, GeneratedFile)); strings.Contains(source, "Models()") {
		t.Fatalf("the generated file holds a Models method\n%s", source)
	}
}

// write puts one source file on the disk.
func write(t *testing.T, path, source string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}

// read returns the content of one file.
func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
