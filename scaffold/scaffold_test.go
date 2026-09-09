package scaffold_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
			".claude/skills/add-slice/SKILL.md", ".env.example", ".gitignore",
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

// skillName is the name that the Agent Skills format accepts.
var skillName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// frontmatter returns the name and the description of a skill.
func frontmatter(t *testing.T, body string) (name, description string) {
	t.Helper()
	if !strings.HasPrefix(body, "---\n") {
		t.Fatalf("the skill carries no frontmatter:\n%s", body)
	}
	end := strings.Index(body[4:], "\n---\n")
	if end < 0 {
		t.Fatalf("the frontmatter does not close:\n%s", body)
	}
	for _, line := range strings.Split(body[4:4+end], "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "name":
			name = strings.TrimSpace(value)
		case "description":
			description = strings.TrimSpace(value)
		}
	}
	return name, description
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

func TestTheAgentFilesStandInEveryShape(t *testing.T) {
	// S16 part B. Every shape carries the agent files, and each skill states
	// the steps, the files and the command that proves the work.
	for _, shape := range []string{scaffold.ShapeSSR, scaffold.ShapeSPA, scaffold.ShapeAPI} {
		dir, _ := write(t, shape)
		agents := readFile(t, filepath.Join(dir, "AGENTS.md"))
		for i := 1; i <= 8; i++ {
			if !strings.Contains(agents, fmt.Sprintf("\n%d. ", i)) {
				t.Fatalf("the %s shape states no design rule %d", shape, i)
			}
		}
		for _, want := range []string{"avero verify", "internal/features/", "Technical English"} {
			if !strings.Contains(agents, want) {
				t.Fatalf("AGENTS.md of the %s shape holds no %q", shape, want)
			}
		}
		for _, skill := range []string{"add-slice", "add-projection", "add-inbox-handler", "add-client"} {
			body := readFile(t, filepath.Join(dir, ".claude", "skills", skill, "SKILL.md"))
			for _, want := range []string{"## Steps", "The command that proves the work"} {
				if !strings.Contains(body, want) {
					t.Fatalf("the skill %s of the %s shape holds no %q", skill, shape, want)
				}
			}
			name, description := frontmatter(t, body)
			// The Agent Skills format states a name of lower case letters,
			// digits and dashes, and a description that says when to use the
			// skill. An agent reads the two lines to decide.
			if name != skill {
				t.Fatalf("the skill %s names itself %q", skill, name)
			}
			if !skillName.MatchString(name) || len(name) > 64 {
				t.Fatalf("the name %q does not follow the format", name)
			}
			if len(description) < 40 || len(description) > 1024 {
				t.Fatalf("the description of %s holds %d characters", skill, len(description))
			}
			if !strings.HasPrefix(description, "Use when") {
				t.Fatalf("the description of %s does not state when to use it: %q", skill, description)
			}
		}
	}
}

func TestWriteSliceWritesTheFeatureAndItsMigration(t *testing.T) {
	dir, _ := write(t, scaffold.ShapeAPI)
	written, err := scaffold.WriteSlice(scaffold.SliceOptions{Name: "Comments", Dir: dir})
	if err != nil {
		t.Fatalf("WriteSlice returned %v, want nil", err)
	}
	if len(written) != 7 {
		t.Fatalf("WriteSlice wrote %v", written)
	}
	body := readFile(t, filepath.Join(dir, "internal", "features", "comments", "module.go"))
	if !strings.Contains(body, "package comments") || !strings.Contains(body, `r.Get("/comments"`) {
		t.Fatalf("module.go holds %q", body)
	}
	if strings.Contains(body, "[[") {
		t.Fatalf("module.go holds an unrendered marker:\n%s", body)
	}

	registered, err := scaffold.RegisterSlice(dir, "example.test/blog", "comments")
	if err != nil {
		t.Fatalf("RegisterSlice returned %v, want nil", err)
	}
	if !registered {
		t.Fatal("RegisterSlice changed no file")
	}
	wire := readFile(t, filepath.Join(dir, "wire.go"))
	if !strings.Contains(wire, "comments.New(engine)") || !strings.Contains(wire, `"example.test/blog/internal/features/comments"`) {
		t.Fatalf("wire.go holds %q", wire)
	}
	// A second registration changes nothing, because the module already
	// stands in the file.
	again, err := scaffold.RegisterSlice(dir, "example.test/blog", "comments")
	if err != nil || again {
		t.Fatalf("RegisterSlice returned %v and %v, want no change", again, err)
	}
}

func TestWriteSliceRefusesASliceThatExists(t *testing.T) {
	dir, _ := write(t, scaffold.ShapeAPI)
	if _, err := scaffold.WriteSlice(scaffold.SliceOptions{Name: "comment", Dir: dir}); err != nil {
		t.Fatalf("WriteSlice returned %v, want nil", err)
	}
	_, err := scaffold.WriteSlice(scaffold.SliceOptions{Name: "comment", Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("WriteSlice returned %v, want a fault", err)
	}
}
