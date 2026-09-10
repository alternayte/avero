package mcp

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Fault is one fault that explain_error found.
type Fault struct {
	// Kind is compile, avero or unknown.
	Kind string `json:"kind"`
	// File names the file of the fault.
	File string `json:"file"`
	// Line names the line of the fault.
	Line int `json:"line"`
	// Column names the column of the fault.
	Column int `json:"column"`
	// Message states the fault.
	Message string `json:"message"`
	// Repair states what to do. It is one sentence.
	Repair string `json:"repair"`
	// Generated reports a fault that a generated file raised.
	Generated bool `json:"generated"`
	// SourceFile names the file that the person wrote, for a fault of a
	// generated file. See DX-6.
	SourceFile string `json:"source_file"`
	// SourceLine names the line that the person wrote.
	SourceLine int `json:"source_line"`
}

// The kinds of fault.
const (
	// KindCompile is a fault of the Go compiler or of go vet.
	KindCompile = "compile"
	// KindAvero is a fault that Avero printed, with its repair.
	KindAvero = "avero"
	// KindUnknown is a line that names no position.
	KindUnknown = "unknown"
)

// positionLine reads a Go position and the message after it. The position can
// stand after a prefix, because `go vet` writes `vet: ./file.go:3:2: …`.
var positionLine = regexp.MustCompile(`^\s*(?:[a-z]+:\s*)?\.?/?([^\s:]+\.go):(\d+)(?::(\d+))?:\s*(.*)$`)

// fieldName reads the field of an input type that a message names, such as
// in.Title.
var fieldName = regexp.MustCompile(`\bin\.([A-Z][A-Za-z0-9_]*)`)

// Explain maps a build fault or a run-time fault to the position in the code
// of the person and to the repair.
//
// A fault that a generated file raised maps back to the input type that the
// person wrote, because an error must never point into generated code. See
// DX-6.
func Explain(dir, message string) map[string]any {
	faults := []Fault{}
	lines := strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "→") {
			// The repair of the line above.
			if len(faults) > 0 {
				faults[len(faults)-1].Kind = KindAvero
				faults[len(faults)-1].Repair = strings.TrimSpace(strings.TrimPrefix(trimmed, "→"))
			}
			continue
		}
		m := positionLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		lineNumber, _ := strconv.Atoi(m[2])
		column, _ := strconv.Atoi(m[3])
		fault := Fault{
			Kind:    KindCompile,
			File:    m[1],
			Line:    lineNumber,
			Column:  column,
			Message: strings.TrimSpace(m[4]),
			Repair:  "Repair the file that the position names. Run `avero verify`.",
		}
		if next := repairOf(lines, i); next != "" {
			fault.Kind, fault.Repair = KindAvero, next
		}
		mapGenerated(dir, &fault)
		faults = append(faults, fault)
	}

	if len(faults) == 0 {
		faults = append(faults, Fault{
			Kind:    KindUnknown,
			Message: strings.TrimSpace(message),
			Repair:  "Send the whole output of the command, because this text names no file and no line",
		})
	}
	return map[string]any{"faults": faults}
}

// repairOf returns the repair that follows a line, or the empty string.
func repairOf(lines []string, i int) string {
	if i+1 >= len(lines) {
		return ""
	}
	next := strings.TrimSpace(lines[i+1])
	if strings.HasPrefix(next, "→") {
		return strings.TrimSpace(strings.TrimPrefix(next, "→"))
	}
	return ""
}

// generatedNames holds the files that the generators write.
var generatedNames = []string{"zz_generated.go", "zz_generated_client.go"}

// mapGenerated maps a fault of a generated file back to the code of the
// person.
func mapGenerated(dir string, fault *Fault) {
	base := filepath.Base(fault.File)
	generated := false
	for _, name := range generatedNames {
		if base == name {
			generated = true
		}
	}
	if !generated {
		return
	}
	fault.Generated = true
	fault.Repair = "Repair the type that the generator reads. Run `avero generate`."

	path := fault.File
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return
	}
	owner := receiverAt(fset, file, fault.Line)
	if owner == "" {
		return
	}

	field := ""
	if m := fieldName.FindStringSubmatch(fault.Message); m != nil {
		field = m[1]
	}
	sourceFile, sourceLine := findType(filepath.Dir(path), owner, field)
	if sourceFile == "" {
		return
	}
	fault.SourceFile, fault.SourceLine = sourceFile, sourceLine
	switch {
	case field != "":
		fault.Repair = fmt.Sprintf("Repair the field %s of %s in %s. Run `avero generate`.",
			field, owner, filepath.Base(sourceFile))
	default:
		fault.Repair = fmt.Sprintf("Repair the type %s in %s. Run `avero generate`.",
			owner, filepath.Base(sourceFile))
	}
}

// receiverAt returns the type of the receiver of the function that holds one
// line of a generated file.
func receiverAt(fset *token.FileSet, file *ast.File, line int) string {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}
		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line
		if line < start || line > end {
			continue
		}
		switch t := fn.Recv.List[0].Type.(type) {
		case *ast.StarExpr:
			if id, ok := t.X.(*ast.Ident); ok {
				return id.Name
			}
		case *ast.Ident:
			return t.Name
		}
	}
	return ""
}

// findType returns the file and the line of a type, or of one of its fields.
func findType(dir, name, field string) (string, int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", 0
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if strings.HasPrefix(entry.Name(), "zz_generated") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		found := ""
		line := 0
		ast.Inspect(file, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok || ts.Name.Name != name {
				return true
			}
			found, line = path, fset.Position(ts.Pos()).Line
			st, ok := ts.Type.(*ast.StructType)
			if !ok || field == "" {
				return false
			}
			for _, f := range st.Fields.List {
				for _, id := range f.Names {
					if id.Name == field {
						line = fset.Position(id.Pos()).Line
					}
				}
			}
			return false
		})
		if found != "" {
			return found, line
		}
	}
	return "", 0
}
