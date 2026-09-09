package client

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GeneratedFile is the name that the generator writes in each package that
// declares a client.
const GeneratedFile = "zz_generated_client.go"

// Generate writes the client of each package under dir that declares one. It
// returns the paths that it wrote, in order. A package with no client
// interface gets no file.
func Generate(dir string) ([]string, error) {
	results, err := build(dir)
	if err != nil {
		return nil, err
	}
	var written []string
	for _, r := range results {
		if err := os.WriteFile(r.path, r.source, 0o600); err != nil {
			return nil, fmt.Errorf("avero generate client: %s did not write: %w", r.path, err)
		}
		written = append(written, r.path)
	}
	return written, nil
}

// Check reports a generated client that is not current. It writes nothing.
func Check(dir string) error {
	results, err := build(dir)
	if err != nil {
		return err
	}
	for _, r := range results {
		onDisk, readErr := os.ReadFile(r.path)
		if readErr != nil {
			return &StaleFault{Path: r.path, Reason: "the file is absent"}
		}
		if !bytes.Equal(onDisk, r.source) {
			return &StaleFault{Path: r.path, Reason: "the file does not match the interface of its package"}
		}
	}
	return nil
}

// StaleFault reports a generated file that a run of the generator would
// change.
type StaleFault struct {
	// Path is the generated file.
	Path string `json:"path"`
	// Reason states what is wrong.
	Reason string `json:"reason"`
}

// Error states the fault and the repair.
func (f *StaleFault) Error() string {
	return fmt.Sprintf("avero generate client: %s is not current: %s\n  → Run `avero generate` and commit the result",
		f.Path, f.Reason)
}

// result is one file that the generator would write.
type result struct {
	path   string
	source []byte
}

// build parses every package under dir and returns the files to write.
func build(dir string) ([]result, error) {
	dirs, err := packageDirs(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	c := &collector{fset: fset}

	var out []result
	for _, d := range dirs {
		byPackage, err := parseDir(fset, d)
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(byPackage))
		for name := range byPackage {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			ifaces := scan(fset, byPackage[name], c)
			if len(ifaces) == 0 {
				continue
			}
			source, emitErr := emit(name, ifaces)
			if emitErr != nil {
				return nil, fmt.Errorf("avero generate client: the output of %s does not format: %w", d, emitErr)
			}
			out = append(out, result{path: filepath.Join(d, GeneratedFile), source: source})
		}
	}
	if err := c.err(); err != nil {
		return nil, err
	}
	return out, nil
}

// parseDir parses the source files of one directory and groups them by package
// name. It reads the files in name order, so that two runs give the same
// output.
func parseDir(fset *token.FileSet, dir string) (map[string][]*ast.File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("avero generate client: %s did not read: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !isSource(e.Name()) {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	out := map[string][]*ast.File{}
	for _, name := range names {
		path := filepath.Join(dir, name)
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
		if parseErr != nil {
			return nil, fmt.Errorf("avero generate client: %s did not parse: %w", path, parseErr)
		}
		out[file.Name.Name] = append(out[file.Name.Name], file)
	}
	return out, nil
}

// isSource reports a file that the person wrote. It skips the generated file
// and every test file.
func isSource(name string) bool {
	return strings.HasSuffix(name, ".go") &&
		name != GeneratedFile &&
		!strings.HasSuffix(name, "_test.go")
}

// packageDirs returns dir and every directory under it that holds Go source.
func packageDirs(dir string) ([]string, error) {
	seen := map[string]bool{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != dir && (name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			seen[filepath.Dir(path)] = true
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("avero generate client: %s did not read: %w", dir, err)
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out, nil
}
