package codegen

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// The generator reads the model package of a feature slice at generation
// time. No generated file imports reflect, and no request path calls it. See
// the SDD, S5, and design rule 2.

// modelImport is the package that the generated Models method calls.
const modelImport = "github.com/alternayte/avero/module"

// model is one persistent model of a feature slice.
type model struct {
	// Name is the name of the Go type.
	Name string
	// Table is the name of the database table, which the meta value of drel
	// states.
	Table string
	// Fields holds the fields of the model: the identifier, then the
	// declared columns, then the two times.
	Fields []modelField
}

// modelField is one field of a model.
type modelField struct {
	// Name is the name of the Go field.
	Name string
	// Type is the type as source text, such as uuid.UUID.
	Type string
}

// modelsOf reads the model directory that stands beside dir and returns its
// models in name order. It returns nothing for a directory that is absent.
func modelsOf(fset *token.FileSet, dir string) ([]model, error) {
	path := filepath.Join(dir, "model")
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return nil, nil
	}
	byPackage, err := parseDir(fset, path)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(byPackage))
	for name := range byPackage {
		names = append(names, name)
	}
	sort.Strings(names)

	var out []model
	for _, name := range names {
		out = append(out, scanModels(fset, byPackage[name])...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// scanModels returns the models of one package.
//
// A model is a struct that embeds drel.Model. The type parameter of the
// embedded type states the type of the identifier, and the meta value of the
// same name states the table.
func scanModels(fset *token.FileSet, files []*ast.File) []model {
	tables := map[string]string{}
	structs := map[string]*ast.StructType{}

	for _, f := range files {
		for _, d := range f.Decls {
			gen, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if st, ok := s.Type.(*ast.StructType); ok {
						structs[s.Name.Name] = st
					}
				case *ast.ValueSpec:
					readTable(s, tables)
				}
			}
		}
	}

	var out []model
	for name, st := range structs {
		table, ok := tables[name]
		if !ok {
			continue
		}
		fields, ok := modelFields(fset, st)
		if !ok {
			continue
		}
		out = append(out, model{Name: name, Table: table, Fields: fields})
	}
	return out
}

// readTable records the table of a meta value, such as
// `var PostMeta = drel.ModelMeta[Post]{Table: "posts"}`. A meta value with
// two type parameters parses as an IndexListExpr. readTable reads the first
// index of either node type as the type name.
func readTable(s *ast.ValueSpec, tables map[string]string) {
	for _, value := range s.Values {
		lit, ok := value.(*ast.CompositeLit)
		if !ok {
			continue
		}
		var x, first ast.Expr
		switch idx := lit.Type.(type) {
		case *ast.IndexExpr:
			x, first = idx.X, idx.Index
		case *ast.IndexListExpr:
			if len(idx.Indices) == 0 {
				continue
			}
			x, first = idx.X, idx.Indices[0]
		default:
			continue
		}
		if !isSelectorNamed(x, "ModelMeta") {
			continue
		}
		name, ok := first.(*ast.Ident)
		if !ok {
			continue
		}
		for _, member := range lit.Elts {
			kv, ok := member.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "Table" {
				continue
			}
			text, ok := kv.Value.(*ast.BasicLit)
			if !ok || text.Kind != token.STRING {
				continue
			}
			if table, err := strconv.Unquote(text.Value); err == nil {
				tables[name.Name] = table
			}
		}
	}
}

// modelFields returns the fields of one model, and it reports a struct that
// embeds drel.Model.
//
// The embedded type gives the identifier and the two times. Its type
// parameter states the type of the identifier.
func modelFields(fset *token.FileSet, st *ast.StructType) ([]modelField, bool) {
	var (
		key      string
		declared []modelField
	)
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			index, ok := f.Type.(*ast.IndexExpr)
			if ok && isSelectorNamed(index.X, "Model") {
				key = exprText(fset, index.Index)
			}
			continue
		}
		if f.Tag == nil {
			continue
		}
		if tag := reflect.StructTag(strings.Trim(f.Tag.Value, "`")); tag.Get("db") == "" {
			continue
		}
		typeText := exprText(fset, f.Type)
		for _, name := range f.Names {
			declared = append(declared, modelField{Name: name.Name, Type: typeText})
		}
	}
	if key == "" {
		return nil, false
	}
	fields := []modelField{{Name: "ID", Type: key}}
	fields = append(fields, declared...)
	fields = append(fields,
		modelField{Name: "CreatedAt", Type: "time.Time"},
		modelField{Name: "UpdatedAt", Type: "time.Time"})
	return fields, true
}
