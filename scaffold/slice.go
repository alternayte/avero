package scaffold

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// DrelConfig is the name of the drel configuration file. drel reads the model
// package of each slice from its modules block.
const DrelConfig = "drel.yaml"

// SliceOptions states one feature slice.
type SliceOptions struct {
	// Name is the name of the feature, such as comment or order.
	Name string
	// Dir is the root of the application.
	Dir string
	// Module is the module path of the application. An empty value reads
	// go.mod.
	Module string
}

// sliceData is the value that a slice template reads.
type sliceData struct {
	// Name is the name of the module, which is the plural form.
	Name string
	// Package is the name of the Go package.
	Package string
	// Singular is the name of one row.
	Singular string
	// Plural is the path segment and the table name.
	Plural string
	// Type is the name of the Go type of one row.
	Type string
	// Table is the name of the database table.
	Table string
	// Module is the module path of the application.
	Module string
}

// WriteSlice writes one feature slice and its migration. It returns the files
// that it wrote, in order.
//
// `avero slice` calls it, and the MCP tool scaffold_slice calls the same
// function, so an agent and a person write the same files. See the SDD, S16.
func WriteSlice(opts SliceOptions) ([]string, error) {
	if opts.Dir == "" {
		opts.Dir = "."
	}
	name := strings.TrimSpace(strings.ToLower(opts.Name))
	if name == "" {
		return nil, faultOf("the slice has no name",
			"Run `avero slice <name>`, such as `avero slice comment`")
	}
	if !validName(name) {
		return nil, faultOf(fmt.Sprintf("the name %q holds a character that a Go package cannot carry", opts.Name),
			"Write a name of letters and digits, such as `comment`")
	}
	module, err := modulePath(opts)
	if err != nil {
		return nil, err
	}

	singular := strings.TrimSuffix(name, "s")
	plural := singular + "s"
	values := sliceData{
		Name:     plural,
		Package:  plural,
		Singular: singular,
		Plural:   plural,
		Type:     upper(singular),
		Table:    plural,
		Module:   module,
	}

	dir := filepath.Join(opts.Dir, "internal", "features", values.Package)
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return nil, faultOf(fmt.Sprintf("the slice %s already exists", values.Package),
			"Give the feature another name, or delete the directory that holds it")
	}

	written, err := renderSlice(dir, values)
	if err != nil {
		return nil, err
	}
	config, err := addSliceModule(opts.Dir, values)
	if err != nil {
		return nil, err
	}
	written = append(written, config...)
	return written, nil
}

// renderSlice writes the Go files of one slice.
func renderSlice(dir string, values sliceData) ([]string, error) {
	entries, err := templates.ReadDir("templates/slice")
	if err != nil {
		return nil, faultOf("the slice templates are absent",
			"Report the fault, because Avero carries the templates")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, faultOf(fmt.Sprintf("the directory %s does not open", dir),
			"Give the process the right to write the application directory")
	}

	var written []string
	for _, entry := range entries {
		if entry.IsDir() {
			files, dirErr := renderSliceDir(dir, entry.Name(), values)
			if dirErr != nil {
				return nil, dirErr
			}
			written = append(written, files...)
			continue
		}
		body, readErr := templates.ReadFile("templates/slice/" + entry.Name())
		if readErr != nil {
			return nil, readErr
		}
		out, renderErr := executeSlice(entry.Name(), string(body), values)
		if renderErr != nil {
			return nil, renderErr
		}
		name := strings.TrimSuffix(entry.Name(), ".tmpl")
		if name == "slice_test.go" {
			name = values.Package + "_test.go"
		}
		target := filepath.Join(dir, name)
		if err := os.WriteFile(target, out, 0o644); err != nil {
			return nil, faultOf(fmt.Sprintf("the file %s does not write", target),
				"Give the process the right to write the application directory")
		}
		written = append(written, target)
	}
	return written, nil
}

// renderSliceDir writes one directory of the slice templates, such as the
// model package that drel reads.
func renderSliceDir(dir, name string, values sliceData) ([]string, error) {
	entries, err := templates.ReadDir("templates/slice/" + name)
	if err != nil {
		return nil, err
	}
	target := filepath.Join(dir, name)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return nil, faultOf(fmt.Sprintf("the directory %s does not open", target),
			"Give the process the right to write the application directory")
	}
	var written []string
	for _, entry := range entries {
		body, readErr := templates.ReadFile("templates/slice/" + name + "/" + entry.Name())
		if readErr != nil {
			return nil, readErr
		}
		out, renderErr := executeSlice(entry.Name(), string(body), values)
		if renderErr != nil {
			return nil, renderErr
		}
		file := strings.TrimSuffix(entry.Name(), ".tmpl")
		if file == "row.go" {
			file = values.Singular + ".go"
		}
		path := filepath.Join(target, file)
		if err := os.WriteFile(path, out, 0o644); err != nil {
			return nil, faultOf(fmt.Sprintf("the file %s does not write", path),
				"Give the process the right to write the application directory")
		}
		written = append(written, path)
	}
	return written, nil
}

// addSliceModule adds the slice to the modules block of drel.yaml.
//
// drel then writes the columns, the repository and the migrations of the
// model package of the slice. The caller runs `drel generate` and
// `drel migrate new` after this call. See the SDD, S14.
func addSliceModule(dir string, values sliceData) ([]string, error) {
	name := filepath.Join(dir, DrelConfig)
	body, err := os.ReadFile(name)
	if err != nil {
		return nil, faultOf("this directory holds no "+DrelConfig,
			"Run the command in the directory of an Avero application, or write one with `avero new`")
	}
	source := string(body)
	if strings.Contains(source, "\n  - name: "+values.Package+"\n") {
		return nil, nil
	}
	marker := "\nmodules:\n"
	i := strings.Index(source, marker)
	if i < 0 {
		return nil, faultOf(DrelConfig+" holds no modules block",
			"Add a `modules:` block to "+DrelConfig+". Run the command again.")
	}
	entry := "  - name: " + values.Package + "\n" +
		"    packages:\n" +
		"      - ./internal/features/" + values.Package + "/model\n" +
		"    migrations: ./internal/features/" + values.Package + "/migrations\n"
	at := i + len(marker)
	source = source[:at] + entry + source[at:]
	if err := os.WriteFile(name, []byte(source), 0o644); err != nil {
		return nil, faultOf(DrelConfig+" does not write",
			"Give the process the right to write the application directory")
	}
	return []string{name}, nil
}

// modulePath returns the module path of one slice.
func modulePath(opts SliceOptions) (string, error) {
	if opts.Module != "" {
		return opts.Module, nil
	}
	return ModulePath(opts.Dir)
}

// ModulePath returns the module path that go.mod names.
func ModulePath(dir string) (string, error) {
	body, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", faultOf("this directory holds no go.mod",
			"Run the command in the directory of an Avero application, or write one with `avero new`")
	}
	for _, line := range strings.Split(string(body), "\n") {
		if after, found := strings.CutPrefix(strings.TrimSpace(line), "module "); found {
			return strings.TrimSpace(after), nil
		}
	}
	return "", faultOf("go.mod names no module",
		"Write the module line in go.mod, such as `module blog`")
}

// addMigrationSet adds the migrations of one slice to migrationSets in
// wire.go. The binary then carries the table of the new feature.
//
// It changes nothing when the function is absent, so a wiring that a person
// wrote by hand stays as it is.
func addMigrationSet(source, module, pkg string) string {
	const marker = "return []fs.FS{"
	i := strings.Index(source, marker)
	if i < 0 {
		return source
	}
	alias := strings.TrimSuffix(pkg, "s") + "migrations"
	if strings.Contains(source, alias+" \"") {
		return source
	}
	at := i + len(marker)
	source = source[:at] + alias + ".FS, " + source[at:]

	importLine := "\t" + alias + " \"" + module + "/internal/features/" + pkg + "/migrations\""
	k := strings.LastIndex(source, "\t\""+module+"/internal/features/")
	if k < 0 {
		return source
	}
	end := strings.Index(source[k:], "\n")
	if end < 0 {
		return source
	}
	end += k + 1
	return source[:end] + importLine + "\n" + source[end:]
}

// upper returns the name with the first letter in upper case.
func upper(name string) string {
	if name == "" {
		return name
	}
	runes := []rune(name)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// RegisterSlice adds the feature to wire.go: the import and the module in the
// call to avero.Modules. It reports whether it changed the file.
//
// It changes nothing when it does not find the two places, so it never breaks
// a wiring that a person wrote by hand. The caller then states the two lines
// to write.
func RegisterSlice(dir, module, pkg string) (bool, error) {
	name := filepath.Join(dir, "wire.go")
	body, err := os.ReadFile(name)
	if err != nil {
		return false, nil
	}
	source := string(body)
	importLine := "\t\"" + module + "/internal/features/" + pkg + "\""
	call := "avero.Modules("
	if strings.Contains(source, importLine) || !strings.Contains(source, call) {
		return false, nil
	}

	marker := "\t\"" + module + "/internal/features/"
	i := strings.LastIndex(source, marker)
	if i < 0 {
		return false, nil
	}
	end := strings.Index(source[i:], "\n")
	if end < 0 {
		return false, nil
	}
	end += i + 1
	source = source[:end] + importLine + "\n" + source[end:]

	j := strings.Index(source, call)
	source = source[:j+len(call)] + pkg + ".New(engine), " + source[j+len(call):]
	source = addMigrationSet(source, module, pkg)

	// The file must read as gofmt writes it, so the import block sorts again.
	out, err := format.Source([]byte(source))
	if err != nil {
		return false, faultOf("wire.go does not format after the registration",
			"Add the module to avero.Modules by hand, and report the fault")
	}
	if err := os.WriteFile(name, out, 0o644); err != nil {
		return false, faultOf("wire.go does not write",
			"Give the process the right to write the application directory")
	}
	return true, nil
}
