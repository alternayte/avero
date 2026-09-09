package scaffold_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/avero/scaffold"
)

// write scaffolds one shape into a temporary directory.
func write(t *testing.T, shape string) (string, []string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "app")
	files, err := scaffold.Write(scaffold.Options{
		Name: "blog", Dir: dir, Shape: shape, Module: "example.test/blog",
	})
	if err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}
	return dir, files
}

func TestEachShapeWritesItsFiles(t *testing.T) {
	for _, tc := range []struct {
		shape string
		want  []string
	}{
		{scaffold.ShapeSSR, []string{
			"main.go", "wire.go", "config.go", "avero.json", "AGENTS.md",
			".avero/skills/add-slice.md", ".env.example", ".gitignore",
			"internal/features/posts/module.go", "internal/ui/ui.go",
			"assets/dist/manifest.json", "acceptance_test.go",
		}},
		{scaffold.ShapeSPA, []string{
			"main.go", "wire.go", "internal/ui/ui.go", "assets/js/app.js",
			"internal/features/posts/handlers.go", "assets/dist/manifest.json",
		}},
		{scaffold.ShapeAPI, []string{
			"main.go", "wire.go", "internal/features/posts/handlers.go",
			"internal/clients/payments/payments.go", "acceptance_test.go",
		}},
	} {
		t.Run(tc.shape, func(t *testing.T) {
			dir, _ := write(t, tc.shape)
			for _, name := range tc.want {
				if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err != nil {
					t.Fatalf("the %s shape wrote no %s", tc.shape, name)
				}
			}
		})
	}
}

func TestNoPlaceholderRemainsInAScaffoldedFile(t *testing.T) {
	// An unrendered marker or a name that stands for another name means that
	// the scaffolder wrote a file that a person must repair by hand.
	//
	// A Markdown file states the form of a command, such as
	// `avero migrate new <name>`, so the angle brackets are prose there and a
	// fault everywhere else.
	always := []string{"[[", "]]", "TODO", "FIXME"}
	code := []string{"<name>", "PLACEHOLDER", "placeholder", "changeme", "myapp"}
	for _, shape := range []string{scaffold.ShapeSSR, scaffold.ShapeSPA, scaffold.ShapeAPI} {
		dir, files := write(t, shape)
		for _, file := range files {
			body, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("ReadFile returned %v", err)
			}
			markers := always
			if !strings.HasSuffix(file, ".md") {
				markers = append(append([]string(nil), always...), code...)
			}
			for _, marker := range markers {
				if strings.Contains(string(body), marker) {
					name := strings.TrimPrefix(file, dir+string(os.PathSeparator))
					t.Fatalf("the %s shape wrote %q in %s", shape, marker, name)
				}
			}
		}
	}
}

func TestTheModulePathReachesEveryImport(t *testing.T) {
	dir, _ := write(t, scaffold.ShapeSSR)
	body, err := os.ReadFile(filepath.Join(dir, "wire.go"))
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}
	if !strings.Contains(string(body), `"example.test/blog/internal/features/posts"`) {
		t.Fatalf("wire.go holds %q", body)
	}
	mod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}
	if !strings.HasPrefix(string(mod), "module example.test/blog") {
		t.Fatalf("go.mod holds %q", mod)
	}
}

func TestTheNameReachesTheDocuments(t *testing.T) {
	dir, _ := write(t, scaffold.ShapeSSR)
	body, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}
	if !strings.Contains(string(body), "# Blog") {
		t.Fatalf("README.md holds %q", body)
	}
}

func TestEachApplicationCarriesItsOwnSecret(t *testing.T) {
	first, _ := write(t, scaffold.ShapeSSR)
	second, _ := write(t, scaffold.ShapeSSR)
	a := readFile(t, filepath.Join(first, ".env.example"))
	b := readFile(t, filepath.Join(second, ".env.example"))
	if a == b {
		t.Fatal("two applications carry one secret")
	}
	if !strings.Contains(a, "AVERO_SECRET=") {
		t.Fatalf(".env.example holds %q", a)
	}
}

func TestAnAPIShapeCarriesNoAdapterAndNoAssets(t *testing.T) {
	dir, _ := write(t, scaffold.ShapeAPI)
	if _, err := os.Stat(filepath.Join(dir, "assets")); err == nil {
		t.Fatal("the api shape wrote an assets directory")
	}
	body := readFile(t, filepath.Join(dir, "avero.json"))
	if strings.Contains(body, "hypermedia") {
		t.Fatalf("avero.json holds %q", body)
	}
}

func TestWriteRefusesADirectoryThatHoldsFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	_, err := scaffold.Write(scaffold.Options{Name: "blog", Dir: dir})
	if err == nil {
		t.Fatal("Write returned nil, want a fault")
	}
	if !strings.Contains(err.Error(), "→") {
		t.Fatalf("the fault states no repair: %v", err)
	}
}

func TestWriteRefusesAnUnknownShapeAndAWrongName(t *testing.T) {
	_, err := scaffold.Write(scaffold.Options{Name: "blog", Dir: t.TempDir(), Shape: "cli"})
	if err == nil || !strings.Contains(err.Error(), "--shape") {
		t.Fatalf("Write returned %v, want a fault that names the flag", err)
	}
	_, err = scaffold.Write(scaffold.Options{Name: "my app", Dir: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "→") {
		t.Fatalf("Write returned %v, want a fault with a repair", err)
	}
	_, err = scaffold.Write(scaffold.Options{Name: "blog", Dir: t.TempDir(), Hypermedia: "turbo"})
	if err == nil || !strings.Contains(err.Error(), "--hypermedia") {
		t.Fatalf("Write returned %v, want a fault that names the flag", err)
	}
}

// readFile returns the content of a file.
func readFile(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}
	return string(body)
}
