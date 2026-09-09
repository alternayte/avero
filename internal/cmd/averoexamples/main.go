// Command averoexamples writes the three reference applications into
// examples/, and proves that they match the templates of the scaffolder.
//
//	go run ./internal/cmd/averoexamples          write the examples again
//	go run ./internal/cmd/averoexamples -check   prove that they are current
//
// The examples are generated output. A person reads them in the repository,
// and the gate proves that they never drift from `avero new`. See the SDD,
// section 8.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/alternayte/avero/codegen"
	"github.com/alternayte/avero/codegen/client"
	"github.com/alternayte/avero/scaffold"
)

// example is one reference application.
type example struct {
	// Name is the name of the application and of its directory.
	Name string
	// Shape is ssr, spa or api.
	Shape string
	// Exercises states what the application proves, for the README.
	Exercises string
}

// examples holds the three reference applications of the SDD, section 8.
var examples = []example{
	{Name: "blog", Shape: "ssr", Exercises: "the router, the handler codegen, the views and the assets"},
	{Name: "board", Shape: "spa", Exercises: "the JSON routes and the embedded front end"},
	{Name: "orders", Shape: "api", Exercises: "the JSON service and the generated client"},
}

// Dir is the directory that holds the examples.
const Dir = "examples"

// skipped names the files that the check does not compare. The assets of a
// build and the sum file are output of a tool, and the example secret is a new
// random value on each run.
var skipped = []string{"assets/dist/", "go.sum"}

// secretLine matches the key of the example environment.
var secretLine = regexp.MustCompile(`(?m)^AVERO_SECRET=.*$`)

// exampleSecret replaces the generated key of an example.
//
// The scaffolder writes a new random key for each application. An example
// stands in a public repository, so its key must be a value that no person
// trusts. It is long enough to start the application, and it states its own
// repair.
const exampleSecret = "AVERO_SECRET=write_a_new_key_with_openssl_rand_hex_32_and_keep_it_secret"

func main() {
	check := flag.Bool("check", false, "prove that the examples match the templates and write nothing")
	flag.Parse()

	if err := run(*check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run writes the examples, or proves them.
func run(check bool) error {
	for _, e := range examples {
		if check {
			if err := prove(e); err != nil {
				return err
			}
			continue
		}
		if err := write(e, filepath.Join(Dir, e.Name), "../.."); err != nil {
			return err
		}
		fmt.Println(filepath.Join(Dir, e.Name))
	}
	return nil
}

// write scaffolds one example into a directory.
func write(e example, dir, replace string) error {
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("the directory %s does not open: %w", dir, err)
	}
	if _, err := scaffold.Write(scaffold.Options{
		Name:    e.Name,
		Dir:     dir,
		Shape:   e.Shape,
		Module:  e.Name,
		Replace: replace,
	}); err != nil {
		return err
	}
	if _, err := codegen.Generate(dir); err != nil {
		return err
	}
	if _, err := client.Generate(dir); err != nil {
		return err
	}
	// The key of an example is a placeholder. See exampleSecret.
	env := filepath.Join(dir, ".env.example")
	if body, err := os.ReadFile(env); err == nil {
		out := secretLine.ReplaceAllString(string(body), exampleSecret)
		if err := os.WriteFile(env, []byte(out), 0o644); err != nil {
			return fmt.Errorf("%s does not write: %w", env, err)
		}
	}

	// The example must build from the repository, so it carries the sum of
	// its dependencies. The check runs the same step, so the two go.mod files
	// hold the same list.
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go mod tidy failed in %s: %w\n%s", dir, err, out)
	}
	return nil
}

// prove writes one example into a temporary directory and compares it with the
// copy in the repository.
func prove(e example) error {
	temp, err := os.MkdirTemp("", "avero-example-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(temp) }()

	fresh := filepath.Join(temp, e.Name)
	root, err := filepath.Abs(".")
	if err != nil {
		return err
	}
	if err := write(e, fresh, root); err != nil {
		return err
	}

	committed := filepath.Join(Dir, e.Name)
	for _, name := range union(list(fresh), list(committed)) {
		want, wantErr := read(filepath.Join(fresh, name))
		got, gotErr := read(filepath.Join(committed, name))
		switch {
		case wantErr != nil:
			return fmt.Errorf("examples: %s holds %s, and the templates write no such file\n  → Run `just examples` and commit the result",
				committed, name)
		case gotErr != nil:
			return fmt.Errorf("examples: %s holds no %s\n  → Run `just examples` and commit the result", committed, name)
		case normalise(want, root) != normalise(got, "../.."):
			return fmt.Errorf("examples: %s does not match the templates\n  → Run `just examples` and commit the result",
				filepath.Join(committed, name))
		}
	}
	return nil
}

// list returns the files of a tree, relative to it, in order.
func list(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		for _, skip := range skipped {
			if rel == skip || strings.HasPrefix(rel, skip) {
				return nil
			}
		}
		out = append(out, rel)
		return nil
	})
	sort.Strings(out)
	return out
}

// union returns every name of the two lists, one time each, in order.
func union(a, b []string) []string {
	seen := map[string]bool{}
	for _, name := range append(append([]string(nil), a...), b...) {
		seen[name] = true
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// read returns the content of a file.
func read(name string) (string, error) {
	body, err := os.ReadFile(name)
	return string(body), err
}

// normalise removes the two values that change on each run: the replace
// directive of the module and the generated secret of the example
// environment.
func normalise(body, replace string) string {
	body = strings.ReplaceAll(body, replace, "<avero>")
	return secretLine.ReplaceAllString(body, "AVERO_SECRET=<key>")
}
