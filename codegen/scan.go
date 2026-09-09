package codegen

import (
	"go/ast"
	"go/printer"
	"go/token"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// The generator reads the struct tags at generation time. reflect.StructTag
// parses one tag string. No generated file imports reflect, and no request
// path calls it. See the SDD, S5, and design rule 2.

// input is one type that a handler takes.
type input struct {
	// Name is the type name.
	Name string
	// Fields holds the fields to bind and to check, in declaration order.
	Fields []field
	// HasCheck reports a Check method on the type, which the generated
	// Validate calls last.
	HasCheck bool
}

// field is one member of an input type.
type field struct {
	// Name is the Go field name.
	Name string
	// Kind classifies the type.
	Kind kind
	// Type is the type as source text, such as int64.
	Type string
	// Path, Query, Form and JSON name the source member. An empty name means
	// that the field does not bind from that source.
	Path, Query, Form, JSON string
	// Rules holds the validation rules in tag order.
	Rules []rule
}

// rule is one validation rule of a field.
type rule struct {
	// Name is required, min, max, email, uuid or oneof.
	Name string
	// Arg is the value after the equals sign.
	Arg string
	// Values holds the members of oneof.
	Values []string
}

// pkg is one parsed package.
type pkg struct {
	Name   string
	Inputs []input
	// Receiver is the type that carries the handlers, such as Module. An
	// empty value states a package with no handler.
	Receiver string
	// Summaries holds the first sentence of the comment of each handler, by
	// the name of its method.
	Summaries map[string]string
}

// scan reads the files of one package and returns the input types that its
// handlers take.
func scan(fset *token.FileSet, name string, files []*ast.File, c *collector) pkg {
	wanted := map[string]bool{}
	checks := map[string]bool{}
	structs := map[string]*ast.StructType{}
	summaries := map[string]string{}
	receiver := ""

	for _, file := range files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if in, ok := handlerInput(d); ok {
					wanted[in] = true
					// The comment of a handler states its summary, so a
					// person writes the sentence one time. See S13.
					if text := summaryOf(d); text != "" {
						summaries[d.Name.Name] = text
					}
					if name, ok := receiverName(d); ok && receiver == "" {
						receiver = name
					}
				}
				if recv, ok := checkReceiver(d); ok {
					checks[recv] = true
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if st, ok := ts.Type.(*ast.StructType); ok {
						structs[ts.Name.Name] = st
					}
				}
			}
		}
	}

	names := make([]string, 0, len(wanted))
	for n := range wanted {
		if _, ok := structs[n]; ok {
			names = append(names, n)
		}
	}
	sort.Strings(names)

	out := pkg{Name: name, Receiver: receiver, Summaries: summaries}
	for _, n := range names {
		out.Inputs = append(out.Inputs, input{
			Name:     n,
			Fields:   readFields(fset, structs[n], c),
			HasCheck: checks[n],
		})
	}
	return out
}

// handlerInput returns the input type of a handler method. A handler is
//
//	func (m *Module) Name(ctx *avero.Ctx, in Input) (View, error)
//
// A handler of a page answers avero.Response instead, because it renders a
// view and names no body. The generator reads the shape and needs no marker
// comment.
func handlerInput(d *ast.FuncDecl) (string, bool) {
	if d.Recv == nil || d.Type.Params == nil || d.Type.Results == nil {
		return "", false
	}
	if len(d.Type.Params.List) != 2 || len(d.Type.Results.List) != 2 {
		return "", false
	}
	if !isPointerTo(d.Type.Params.List[0].Type, "Ctx") {
		return "", false
	}
	in, ok := d.Type.Params.List[1].Type.(*ast.Ident)
	if !ok {
		return "", false
	}
	if !answersAResult(d.Type.Results.List[0].Type) {
		return "", false
	}
	if id, ok := d.Type.Results.List[1].Type.(*ast.Ident); !ok || id.Name != "error" {
		return "", false
	}
	return in.Name, true
}

// answersAResult reports the first result of a handler.
//
// A handler answers the thing that it returns, so the type is free. The second
// result must be error, which the caller proves, and the first parameter must
// be *avero.Ctx. Those two hold the shape, so a method of a module that is no
// handler does not match.
func answersAResult(expr ast.Expr) bool {
	switch expr.(type) {
	case *ast.IndexExpr, *ast.Ident, *ast.SelectorExpr, *ast.StarExpr, *ast.ArrayType:
		return true
	default:
		return false
	}
}

// checkReceiver returns the type of a custom validation hook. The hook is
//
//	func (in Input) Check(c *avero.Ctx, f *avero.Fields)
func checkReceiver(d *ast.FuncDecl) (string, bool) {
	if d.Recv == nil || d.Name.Name != "Check" || d.Type.Params == nil {
		return "", false
	}
	if d.Type.Results != nil && len(d.Type.Results.List) != 0 {
		return "", false
	}
	if len(d.Type.Params.List) != 2 {
		return "", false
	}
	if !isPointerTo(d.Type.Params.List[0].Type, "Ctx") ||
		!isPointerTo(d.Type.Params.List[1].Type, "Fields") {
		return "", false
	}
	if len(d.Recv.List) != 1 {
		return "", false
	}
	switch r := d.Recv.List[0].Type.(type) {
	case *ast.Ident:
		return r.Name, true
	case *ast.StarExpr:
		if id, ok := r.X.(*ast.Ident); ok {
			return id.Name, true
		}
	}
	return "", false
}

// isPointerTo reports a *pkg.Name expression.
func isPointerTo(expr ast.Expr, name string) bool {
	star, ok := expr.(*ast.StarExpr)
	return ok && isSelectorNamed(star.X, name)
}

// isSelectorNamed reports a pkg.Name expression. The package qualifier is free,
// because avero.Ctx and router.Ctx are the same type.
func isSelectorNamed(expr ast.Expr, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == name
}

// readFields reads every field of an input struct and records a fault for each
// one that the generator cannot bind.
func readFields(fset *token.FileSet, st *ast.StructType, c *collector) []field {
	var out []field
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			c.at(f.Pos(), "an embedded field is not supported inside an input type",
				"Write the fields of the embedded struct into the input type")
			continue
		}
		typeText := exprText(fset, f.Type)
		for _, name := range f.Names {
			if !name.IsExported() {
				continue
			}
			out = append(out, readField(name, typeText, f, c)...)
		}
	}
	return out
}

// readField reads one named field.
func readField(name *ast.Ident, typeText string, f *ast.Field, c *collector) []field {
	tag := reflect.StructTag("")
	if f.Tag != nil {
		if unquoted, err := strconv.Unquote(f.Tag.Value); err == nil {
			tag = reflect.StructTag(unquoted)
		}
	}
	if tag.Get("avero") == "-" {
		return nil
	}

	k, ok := classify(typeText)
	if !ok {
		c.at(name.Pos(),
			"the field "+name.Name+" has type "+typeText+", which the generator cannot bind",
			"Change the type of "+name.Name+" to a supported type, or mark it `avero:\"-\"`")
		return nil
	}

	fl := field{Name: name.Name, Kind: k, Type: typeText}
	fl.Path = tagName(tag, "path", name.Name)
	fl.Query = tagName(tag, "query", name.Name)
	fl.Form = tagName(tag, "form", name.Name)
	fl.JSON = jsonName(tag)

	if fl.Path == "" && fl.Query == "" && fl.Form == "" && fl.JSON == "" {
		c.at(name.Pos(),
			"the field "+name.Name+" binds from no source",
			"Add a path, query, form or json tag to "+name.Name+", or mark it `avero:\"-\"`")
		return nil
	}
	fl.Rules = readRules(tag.Get("validate"), name, k, c)
	return []field{fl}
}

// tagName returns the member name of one source. An empty tag means that the
// field does not bind from that source. A tag with no value takes the field
// name in snake case.
func tagName(tag reflect.StructTag, key, fieldName string) string {
	raw, ok := tag.Lookup(key)
	if !ok {
		return ""
	}
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "," {
		return snake(fieldName)
	}
	if i := strings.IndexByte(raw, ','); i >= 0 {
		raw = raw[:i]
	}
	if raw == "" {
		return snake(fieldName)
	}
	return raw
}

// jsonName returns the body member name. A field with no json tag does not
// bind from the body, because encoding/json would then match it by field name
// and hide the fault of a missing tag.
func jsonName(tag reflect.StructTag) string {
	raw, ok := tag.Lookup("json")
	if !ok {
		return ""
	}
	if i := strings.IndexByte(raw, ','); i >= 0 {
		raw = raw[:i]
	}
	if raw == "-" {
		return ""
	}
	return raw
}

// exprText prints a type expression as source text.
func exprText(fset *token.FileSet, expr ast.Expr) string {
	var b strings.Builder
	if err := printer.Fprint(&b, fset, expr); err != nil {
		return ""
	}
	return b.String()
}

// snake turns a field name into snake case. Title becomes title, and BoardID
// becomes board_id.
func snake(name string) string {
	runes := []rune(name)
	var b strings.Builder
	for i, r := range runes {
		upper := r >= 'A' && r <= 'Z'
		if i > 0 && upper {
			prev := runes[i-1]
			prevUpper := prev >= 'A' && prev <= 'Z'
			nextLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z'
			if !prevUpper || nextLower {
				b.WriteByte('_')
			}
		}
		if upper {
			r = r - 'A' + 'a'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// receiverName returns the type of the receiver of a method, without its
// pointer.
func receiverName(d *ast.FuncDecl) (string, bool) {
	if d.Recv == nil || len(d.Recv.List) != 1 {
		return "", false
	}
	switch r := d.Recv.List[0].Type.(type) {
	case *ast.StarExpr:
		if id, ok := r.X.(*ast.Ident); ok {
			return id.Name, true
		}
	case *ast.Ident:
		return r.Name, true
	}
	return "", false
}

// summaryOf returns the first sentence of the comment of a handler.
//
// The sentence loses the name of the method and the full stop, because a
// description of an API reads "List every post" and not "List returns every
// post.".
func summaryOf(d *ast.FuncDecl) string {
	if d.Doc == nil {
		return ""
	}
	var lines []string
	for _, line := range d.Doc.List {
		text := strings.TrimSpace(strings.TrimPrefix(line.Text, "//"))
		if text == "" {
			break
		}
		if strings.HasPrefix(text, "avero:") || strings.HasPrefix(text, "go:") {
			continue
		}
		lines = append(lines, text)
	}
	summary := strings.Join(lines, " ")
	if i := strings.Index(summary, ". "); i > 0 {
		summary = summary[:i+1]
	}
	summary = strings.TrimSuffix(summary, ".")
	rest, cut := strings.CutPrefix(summary, d.Name.Name+" ")
	if !cut {
		return strings.TrimSpace(summary)
	}
	// "List answers every post." states "Answers every post", because a
	// description of an API names the operation beside its summary.
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return ""
	}
	runes := []rune(rest)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
