package openapi

import (
	"go/ast"
	"strconv"
	"strings"
)

// pkg is one package of the application that registers routes.
type pkg struct {
	// name is the name of the package, which names the module.
	name string
	// routes holds the routes that the Routes method registers, in order.
	routes []route
	// handlers maps the name of a handler to its shape.
	handlers map[string]method
	// structs maps the name of an input type to its fields.
	structs map[string]structType
}

// route is one registration inside a Routes method.
type route struct {
	// Method is the HTTP method.
	Method string
	// Path is the pattern of the route.
	Path string
	// Handler is the name of the method that serves the route.
	Handler string
}

// method is one handler of a module.
type method struct {
	// Name is the name of the method.
	Name string
	// Input is the name of the input type.
	Input string
	// Summary is the first sentence of the comment of the method.
	Summary string
	// Answers holds the answers that the directives of the method state.
	Answers []answer
}

// answer is one //avero:response directive of a handler.
//
//	//avero:response 200 TaskList
//	//avero:response 201 Task
//	//avero:response 200 []Task
type answer struct {
	// Code is the status of the answer.
	Code string
	// Type is the name of the Go type of the body. An empty name states an
	// answer with no body.
	Type string
	// List states an answer that carries an array of the type.
	List bool
}

// structType is one input type.
type structType struct {
	// fields holds one entry for each field with a source tag.
	fields []field
	// validated reports a type that carries a validate tag, so the router
	// answers 422 for it.
	validated bool
}

// field is one field of an input type.
type field struct {
	// Name is the Go field name.
	Name string
	// Path, Query, Header, Form and JSON name the source member.
	Path, Query, Header, Form, JSON string
	// JSONType is the type of the value in a JSON document.
	JSONType string
	// Named is the Go type of a field that names a type of the package, or
	// the member of such an array.
	Named string
	// ItemType is the type of the member of an array of plain values.
	ItemType string
	// Format states the shape of a string, such as date-time.
	Format string
	// Required states a field that the request must carry.
	Required bool
	// Optional states a field of an answer that the JSON tag marks with
	// omitempty, so the answer can leave it out.
	Optional bool
	// Rules holds the validation rules of the field.
	Rules []rule
}

// rule is one validation rule.
type rule struct {
	// Name is required, min, max, email, uuid or oneof.
	Name string
	// Arg is the value after the equals sign.
	Arg string
	// Values holds the members of oneof.
	Values []string
}

// scanPackage reads the routes, the handlers and the input types of one
// package.
func scanPackage(name string, files []*ast.File) pkg {
	p := pkg{name: name, handlers: map[string]method{}, structs: map[string]structType{}}
	for _, file := range files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				// A module registers its routes in a Routes method, and the
				// composition root registers its own in wire.go, so every
				// function is read.
				p.routes = append(p.routes, routesOf(d)...)
				if m, ok := handlerOf(d); ok {
					p.handlers[m.Name] = m
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					st, ok := ts.Type.(*ast.StructType)
					if !ok {
						continue
					}
					p.structs[ts.Name.Name] = fieldsOf(st)
				}
			}
		}
	}
	return p
}

// routesOf reads the registrations of one function.
//
//	r.Get("/posts/{id}", avero.In(m.Show))
func routesOf(fn *ast.FuncDecl) []route {
	var out []route
	if fn.Body == nil {
		return nil
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		httpMethod, ok := methods[selector.Sel.Name]
		if !ok {
			return true
		}
		pattern, ok := stringOf(call.Args[0])
		if !ok {
			return true
		}
		// A route that carries a function of its own has no name and no
		// input type. The description still holds it, because a client calls
		// it.
		handler := handlerName(call.Args[1])
		out = append(out, route{Method: httpMethod, Path: openAPIPath(pattern), Handler: handler})
		return true
	})
	return out
}

// methods maps the method of the router to the HTTP method.
var methods = map[string]string{
	"Get": "GET", "Post": "POST", "Put": "PUT", "Patch": "PATCH",
	"Delete": "DELETE", "Head": "HEAD", "Options": "OPTIONS",
}

// stringOf returns the value of a string literal.
func stringOf(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind.String() != "STRING" {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}

// handlerName returns the name of the handler that a registration names.
//
//	avero.In(m.Show)   Show
//	m.Show             Show
func handlerName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.CallExpr:
		if len(e.Args) != 1 {
			return ""
		}
		return handlerName(e.Args[0])
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.Ident:
		return e.Name
	default:
		return ""
	}
}

// openAPIPath turns a pattern of the router into a path of the description.
// The root pattern of Go, /{$}, is the root path.
func openAPIPath(pattern string) string {
	pattern = strings.ReplaceAll(pattern, "/{$}", "/")
	if pattern == "" {
		return "/"
	}
	return pattern
}

// handlerOf reads one handler of a module.
//
//	func (m *Module) Show(c *avero.Ctx, in ShowInput) (avero.Response, error)
func handlerOf(fn *ast.FuncDecl) (method, bool) {
	if fn.Recv == nil || fn.Type.Params == nil || fn.Type.Results == nil {
		return method{}, false
	}
	if len(fn.Type.Params.List) != 2 || len(fn.Type.Results.List) != 2 {
		return method{}, false
	}
	// The shape is the one that the generator of S5 reads: a pointer to a Ctx,
	// one input type, a Response and an error. A method of a store carries two
	// parameters as well, so the test reads the types.
	if !isPointerTo(fn.Type.Params.List[0].Type, "Ctx") {
		return method{}, false
	}
	if !isSelectorNamed(fn.Type.Results.List[0].Type, "Response") {
		return method{}, false
	}
	if id, ok := fn.Type.Results.List[1].Type.(*ast.Ident); !ok || id.Name != "error" {
		return method{}, false
	}
	input, ok := fn.Type.Params.List[1].Type.(*ast.Ident)
	if !ok {
		return method{}, false
	}
	return method{
		Name:    fn.Name.Name,
		Input:   input.Name,
		Summary: summaryOf(fn),
		Answers: answersOf(fn),
	}, true
}

// isPointerTo reports a *pkg.Name expression.
func isPointerTo(expr ast.Expr, name string) bool {
	star, ok := expr.(*ast.StarExpr)
	return ok && isSelectorNamed(star.X, name)
}

// isSelectorNamed reports a pkg.Name expression. The qualifier is free,
// because avero.Ctx and router.Ctx are the same type.
func isSelectorNamed(expr ast.Expr, name string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == name
}

// ResponseDirective states the answer of a handler.
const ResponseDirective = "//avero:response"

// answersOf reads the response directives of a handler.
func answersOf(fn *ast.FuncDecl) []answer {
	if fn.Doc == nil {
		return nil
	}
	var out []answer
	for _, line := range fn.Doc.List {
		text := strings.TrimSpace(line.Text)
		if !strings.HasPrefix(text, ResponseDirective) {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(text, ResponseDirective))
		if len(fields) == 0 {
			continue
		}
		a := answer{Code: fields[0]}
		if len(fields) > 1 {
			name := fields[1]
			if after, found := strings.CutPrefix(name, "[]"); found {
				a.List, name = true, after
			}
			a.Type = name
		}
		out = append(out, a)
	}
	return out
}

// summaryOf returns the first sentence of the comment of a function.
func summaryOf(fn *ast.FuncDecl) string {
	if fn.Doc == nil {
		return ""
	}
	var lines []string
	for _, line := range fn.Doc.List {
		text := strings.TrimSpace(strings.TrimPrefix(line.Text, "//"))
		if text == "" {
			break
		}
		if strings.HasPrefix(strings.TrimSpace(line.Text), "//avero:") {
			continue
		}
		lines = append(lines, text)
	}
	summary := strings.Join(lines, " ")
	if i := strings.Index(summary, ". "); i > 0 {
		summary = summary[:i+1]
	}
	summary = strings.TrimSuffix(summary, ".")
	return strings.TrimPrefix(summary, fn.Name.Name+" ")
}

// fieldsOf reads the fields of one input type.
func fieldsOf(st *ast.StructType) structType {
	out := structType{}
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 || f.Tag == nil {
			continue
		}
		tag, err := strconv.Unquote(f.Tag.Value)
		if err != nil {
			continue
		}
		entry := field{Name: f.Names[0].Name}
		entry.Path, _ = lookupTag(tag, "path")
		entry.Query, _ = lookupTag(tag, "query")
		entry.Header, _ = lookupTag(tag, "header")
		entry.Form, _ = lookupTag(tag, "form")
		entry.JSON, _ = lookupTag(tag, "json")
		entry.Path = memberName(entry.Path, entry.Name)
		entry.Query = memberName(entry.Query, entry.Name)
		entry.Header = memberName(entry.Header, entry.Name)
		entry.Form = memberName(entry.Form, entry.Name)
		entry.JSON = memberName(entry.JSON, entry.Name)
		if entry.Path == "" && entry.Query == "" && entry.Header == "" && entry.Form == "" && entry.JSON == "" {
			continue
		}
		entry.JSONType, entry.Format = jsonType(f.Type)
		entry.Named, entry.ItemType = namedType(f.Type)
		if value, found := lookupTag(tag, "json"); found {
			_, options, _ := strings.Cut(value, ",")
			entry.Optional = strings.Contains(options, "omitempty")
		}
		if rules, found := lookupTag(tag, "validate"); found {
			entry.Rules = readRules(rules)
			out.validated = true
			for _, r := range entry.Rules {
				if r.Name == "required" {
					entry.Required = true
				}
			}
		}
		out.fields = append(out.fields, entry)
	}
	return out
}

// memberName returns the name that a source tag states, or the empty string.
// A tag of a dash names no member.
func memberName(value, goName string) string {
	if value == "" {
		return ""
	}
	name, _, _ := strings.Cut(value, ",")
	if name == "-" {
		return ""
	}
	if name == "" {
		return goName
	}
	return name
}

// namedType returns the name of the type that a field states, when the package
// holds it, and the type of the member of an array of plain values.
func namedType(expr ast.Expr) (named, item string) {
	switch e := expr.(type) {
	case *ast.Ident:
		if plain(e.Name) {
			return "", ""
		}
		return e.Name, ""
	case *ast.StarExpr:
		return namedType(e.X)
	case *ast.ArrayType:
		inner, _ := namedType(e.Elt)
		if inner != "" {
			return inner, ""
		}
		kind, _ := jsonType(e.Elt)
		return "", kind
	default:
		return "", ""
	}
}

// plain reports a type of the language, which needs no schema of its own.
func plain(name string) bool {
	switch name {
	case "string", "bool", "byte", "rune", "error", "any",
		"float32", "float64",
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64":
		return true
	default:
		return false
	}
}

// jsonType returns the type of a value in a JSON document, and its format.
func jsonType(expr ast.Expr) (string, string) {
	switch e := expr.(type) {
	case *ast.Ident:
		switch e.Name {
		case "string":
			return "string", ""
		case "bool":
			return "boolean", ""
		case "float32", "float64":
			return "number", ""
		case "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64":
			return "integer", ""
		default:
			return "string", ""
		}
	case *ast.SelectorExpr:
		switch e.Sel.Name {
		case "Time":
			return "string", "date-time"
		case "Duration":
			return "string", "duration"
		default:
			return "string", ""
		}
	case *ast.ArrayType:
		return "array", ""
	case *ast.StarExpr:
		return jsonType(e.X)
	default:
		return "string", ""
	}
}

// readRules parses one validate tag.
func readRules(tag string) []rule {
	var out []rule
	for _, part := range strings.Split(tag, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		head, arg, _ := strings.Cut(part, "=")
		r := rule{Name: strings.TrimSpace(head), Arg: strings.TrimSpace(arg)}
		if r.Name == "oneof" {
			r.Values = strings.Fields(r.Arg)
		}
		out = append(out, r)
	}
	return out
}
