package client

import (
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The directive prefixes that the generator reads.
const (
	// ClientDirective declares an interface as a client.
	ClientDirective = "//avero:client"
	// MethodPrefix starts a method directive, such as //avero:GET.
	MethodPrefix = "//avero:"
)

// methods holds the HTTP methods that a directive can name.
var methods = map[string]bool{
	http.MethodGet: true, http.MethodHead: true, http.MethodPost: true,
	http.MethodPut: true, http.MethodPatch: true, http.MethodDelete: true,
	http.MethodOptions: true,
}

// idempotent holds the methods that a retry can repeat.
var idempotent = map[string]bool{
	http.MethodGet: true, http.MethodHead: true, http.MethodPut: true,
	http.MethodDelete: true, http.MethodOptions: true,
}

// iface is one interface that carries a client directive.
type iface struct {
	// Name is the name of the interface.
	Name string
	// Base is the address of the service.
	Base string
	// Auth is the authentication shape.
	Auth string
	// Timeout, Attempts and Backoff hold the options of the directive.
	Timeout  string
	Attempts string
	Backoff  string
	// Methods holds one entry for each method of the interface.
	Methods []method
}

// method is one call.
type method struct {
	// Name is the name of the method.
	Name string
	// HTTP is the HTTP method.
	HTTP string
	// Path is the path of the directive, with its placeholders.
	Path string
	// Idempotent states that a retry is safe.
	Idempotent bool
	// PathArgs holds one argument for each placeholder, in order.
	PathArgs []arg
	// Query holds the query fields of the option parameter.
	Query []field
	// Header holds the header fields of the option parameter.
	Header []field
	// Option is the name of the parameter that holds Query and Header.
	Option string
	// OptionType is the type of that parameter, as the interface writes it.
	OptionType string
	// Body is the name of the parameter that becomes the JSON body.
	Body string
	// BodyType is the type of that parameter, as the interface writes it.
	BodyType string
	// Result is the type of the first result, or the empty string.
	Result string
	// ResultPointer states a result that is already a pointer.
	ResultPointer bool
}

// arg is one path parameter.
type arg struct {
	// Name is the name of the Go parameter.
	Name string
	// Type is the type of the parameter, as the interface writes it.
	Type string
	// Kind states how the generated code turns it into a string.
	Kind kind
}

// field is one query field or one header field.
type field struct {
	// Go is the name of the struct field.
	Go string
	// Wire is the name in the query or in the header.
	Wire string
	// Kind states how the generated code turns it into a string.
	Kind kind
	// Pointer states a pointer field, which the call sends only when it
	// holds a value.
	Pointer bool
	// OmitEmpty states a field that the call sends only when it holds a
	// value.
	OmitEmpty bool
}

// kind is the shape of a value that the call writes as a string.
type kind int

// The kinds that a path parameter, a query field and a header field accept.
const (
	kindString kind = iota
	kindInt
	kindUint
	kindFloat
	kindBool
)

// scan reads one package and returns the interfaces that carry a directive.
func scan(fset *token.FileSet, files []*ast.File, c *collector) []iface {
	structs := map[string]*ast.StructType{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
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

	var out []iface
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				it, ok := ts.Type.(*ast.InterfaceType)
				if !ok {
					continue
				}
				line := directive(doc(gen, ts), ClientDirective)
				if line == nil {
					continue
				}
				out = append(out, scanInterface(fset, ts, it, *line, structs, c))
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// doc returns the comment group of a type, which the declaration holds when it
// stands alone.
func doc(gen *ast.GenDecl, ts *ast.TypeSpec) *ast.CommentGroup {
	if ts.Doc != nil {
		return ts.Doc
	}
	return gen.Doc
}

// comment is one directive line with its position.
type comment struct {
	Text string
	Pos  token.Pos
}

// directive returns the first line of a comment group that starts with the
// prefix.
func directive(group *ast.CommentGroup, prefix string) *comment {
	if group == nil {
		return nil
	}
	for _, line := range group.List {
		if strings.HasPrefix(line.Text, prefix) {
			return &comment{Text: line.Text, Pos: line.Pos()}
		}
	}
	return nil
}

// duration proves that a value is a Go duration, such as 5s.
func duration(c *collector, line comment, p pair) string {
	if _, err := time.ParseDuration(p.value); err != nil {
		c.at(line.Pos, fmt.Sprintf("the %s %q is not a duration", p.key, p.value),
			fmt.Sprintf("Write %s=\"5s\", with a unit that time.ParseDuration reads", p.key))
		return ""
	}
	return p.value
}

// scanInterface reads one interface and its methods.
func scanInterface(fset *token.FileSet, ts *ast.TypeSpec, it *ast.InterfaceType,
	line comment, structs map[string]*ast.StructType, c *collector,
) iface {
	in := iface{Name: ts.Name.Name, Auth: string(AuthNone)}
	pairs, err := options(strings.TrimPrefix(line.Text, ClientDirective))
	if err != nil {
		c.at(line.Pos, err.Error(),
			"Write each option as key=\"value\", such as base=\"https://api.example.com\"")
		return in
	}
	for _, p := range pairs {
		switch p.key {
		case "base":
			in.Base = p.value
		case "auth":
			switch Auth(p.value) {
			case AuthNone, AuthBearer, AuthBasic:
				in.Auth = p.value
			default:
				c.at(line.Pos, fmt.Sprintf("the auth %q is not known", p.value),
					"Write auth=\"bearer\", auth=\"basic\" or auth=\"none\"")
			}
		case "timeout":
			in.Timeout = duration(c, line, p)
		case "attempts":
			if _, err := strconv.Atoi(p.value); err != nil {
				c.at(line.Pos, fmt.Sprintf("the attempts %q is not a number", p.value),
					"Write attempts=\"3\", which counts the first attempt")
				break
			}
			in.Attempts = p.value
		case "backoff":
			in.Backoff = duration(c, line, p)
		default:
			c.at(line.Pos, fmt.Sprintf("the option %q is not known", p.key),
				"Write base, auth, timeout, attempts or backoff")
		}
	}
	if in.Base == "" {
		c.at(line.Pos, fmt.Sprintf("the client %s states no base address", in.Name),
			"Write base=\"https://api.example.com\" in the directive")
	}

	for _, item := range it.Methods.List {
		fn, ok := item.Type.(*ast.FuncType)
		if !ok || len(item.Names) == 0 {
			c.at(item.Pos(), "a client interface holds methods only",
				"Delete the embedded interface, or move it out of the client interface")
			continue
		}
		name := item.Names[0].Name
		line := methodDirective(item)
		if line == nil {
			c.at(item.Pos(), fmt.Sprintf("the method %s carries no directive", name),
				"Write //avero:GET /things/{id} above the method, with the method and the path of the call")
			continue
		}
		m, ok := scanMethod(fset, name, *line, fn, structs, c)
		if ok {
			in.Methods = append(in.Methods, m)
		}
	}
	return in
}

// methodDirective returns the avero directive of one method.
func methodDirective(item *ast.Field) *comment {
	if item.Doc == nil {
		return nil
	}
	for _, line := range item.Doc.List {
		if strings.HasPrefix(line.Text, MethodPrefix) && !strings.HasPrefix(line.Text, ClientDirective) {
			return &comment{Text: line.Text, Pos: line.Pos()}
		}
	}
	return nil
}

// scanMethod reads one method and its directive.
func scanMethod(fset *token.FileSet, name string, line comment, fn *ast.FuncType,
	structs map[string]*ast.StructType, c *collector,
) (method, bool) {
	rest := strings.TrimPrefix(line.Text, MethodPrefix)
	parts := strings.Fields(rest)
	if len(parts) < 2 {
		c.at(line.Pos, fmt.Sprintf("the directive of %s names no method and no path", name),
			"Write //avero:GET /things/{id}, with the HTTP method and the path")
		return method{}, false
	}
	m := method{Name: name, HTTP: strings.ToUpper(parts[0]), Path: parts[1]}
	if !methods[m.HTTP] {
		c.at(line.Pos, fmt.Sprintf("the HTTP method %q is not known", parts[0]),
			"Write GET, HEAD, POST, PUT, PATCH, DELETE or OPTIONS")
		return method{}, false
	}
	if !strings.HasPrefix(m.Path, "/") {
		c.at(line.Pos, fmt.Sprintf("the path %q does not start with a slash", m.Path),
			fmt.Sprintf("Write the path as `/%s`", strings.TrimPrefix(m.Path, "/")))
		return method{}, false
	}
	m.Idempotent = idempotent[m.HTTP]
	if len(parts) > 2 {
		pairs, err := options(strings.Join(parts[2:], " "))
		if err != nil {
			c.at(line.Pos, err.Error(),
				"Write each option as key=\"value\", such as idempotent=\"true\"")
			return method{}, false
		}
		for _, p := range pairs {
			if p.key != "idempotent" {
				c.at(line.Pos, fmt.Sprintf("the option %q is not known", p.key),
					"Write idempotent=\"true\" to allow a retry of a method that carries a body")
				continue
			}
			m.Idempotent = p.value == "true"
		}
	}
	if !scanParams(fset, &m, line, fn, structs, c) {
		return method{}, false
	}
	if !scanResults(fset, &m, line, fn, c) {
		return method{}, false
	}
	return m, true
}

// scanParams reads the parameters of one method.
func scanParams(fset *token.FileSet, m *method, line comment, fn *ast.FuncType,
	structs map[string]*ast.StructType, c *collector,
) bool {
	params := flatten(fn.Params)
	if len(params) == 0 || exprString(fset, params[0].Type) != "context.Context" {
		c.at(line.Pos, fmt.Sprintf("the first parameter of %s is not a context.Context", m.Name),
			"Write ctx context.Context as the first parameter of the method")
		return false
	}
	params = params[1:]

	placeholders := pathNames(m.Path)
	byName := map[string]namedParam{}
	for _, p := range params {
		byName[strings.ToLower(p.Name)] = p
	}
	used := map[string]bool{}
	ok := true
	for _, ph := range placeholders {
		p, found := byName[strings.ToLower(ph)]
		if !found {
			c.at(line.Pos, fmt.Sprintf("the path of %s names {%s} and the method has no parameter %s", m.Name, ph, ph),
				fmt.Sprintf("Add the parameter %s to the method, or delete {%s} from the path", ph, ph))
			ok = false
			continue
		}
		k, valid := kindOf(exprString(fset, p.Type))
		if !valid {
			c.at(p.Pos, fmt.Sprintf("the path parameter %s has the type %s, which a path cannot carry",
				p.Name, exprString(fset, p.Type)),
				"Give the parameter a string type, an integer type, a floating point type or a bool")
			ok = false
			continue
		}
		m.PathArgs = append(m.PathArgs, arg{Name: p.Name, Type: exprString(fset, p.Type), Kind: k})
		used[p.Name] = true
	}

	for _, p := range params {
		if used[p.Name] {
			continue
		}
		typeName := strings.TrimPrefix(exprString(fset, p.Type), "*")
		st, isLocal := structs[typeName]
		switch {
		case isLocal && tagged(st):
			if m.Option != "" {
				c.at(p.Pos, fmt.Sprintf("the method %s takes two option parameters", m.Name),
					"Join the query fields and the header fields into one struct")
				ok = false
				continue
			}
			m.Option = p.Name
			m.OptionType = exprString(fset, p.Type)
			if !scanOption(fset, m, st, c) {
				ok = false
			}
		default:
			if m.Body != "" {
				c.at(p.Pos, fmt.Sprintf("the method %s takes two bodies", m.Name),
					"Join the values into one struct, or delete the second parameter")
				ok = false
				continue
			}
			if m.HTTP == http.MethodGet || m.HTTP == http.MethodHead {
				c.at(p.Pos, fmt.Sprintf("the method %s is a %s and carries a body", m.Name, m.HTTP),
					"Delete the parameter, or give its struct query tags, or change the HTTP method")
				ok = false
				continue
			}
			m.Body = p.Name
			m.BodyType = exprString(fset, p.Type)
		}
	}
	return ok
}

// scanOption reads the query fields and the header fields of an option struct.
func scanOption(fset *token.FileSet, m *method, st *ast.StructType, c *collector) bool {
	ok := true
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 || f.Tag == nil {
			continue
		}
		tag, err := strconv.Unquote(f.Tag.Value)
		if err != nil {
			continue
		}
		name := f.Names[0].Name
		expr := exprString(fset, f.Type)
		pointer := strings.HasPrefix(expr, "*")
		k, valid := kindOf(strings.TrimPrefix(expr, "*"))

		for _, key := range []string{"query", "header"} {
			value, found := lookupTag(tag, key)
			if !found {
				continue
			}
			if !valid {
				c.at(f.Pos(), fmt.Sprintf("the field %s has the type %s, which a %s cannot carry", name, expr, key),
					"Give the field a string type, an integer type, a floating point type or a bool")
				ok = false
				continue
			}
			wire, opts, _ := strings.Cut(value, ",")
			if wire == "" {
				wire = name
			}
			entry := field{
				Go: name, Wire: wire, Kind: k, Pointer: pointer,
				OmitEmpty: strings.Contains(opts, "omitempty"),
			}
			if key == "query" {
				m.Query = append(m.Query, entry)
			} else {
				m.Header = append(m.Header, entry)
			}
		}
	}
	return ok
}

// scanResults reads the results of one method.
func scanResults(fset *token.FileSet, m *method, line comment, fn *ast.FuncType, c *collector) bool {
	results := flatten(fn.Results)
	repair := "Write (T, error) for a method that reads a value, or error for a method that reads none"
	switch len(results) {
	case 1:
		if exprString(fset, results[0].Type) != "error" {
			c.at(line.Pos, fmt.Sprintf("the method %s returns one value that is not an error", m.Name), repair)
			return false
		}
		return true
	case 2:
		if exprString(fset, results[1].Type) != "error" {
			c.at(line.Pos, fmt.Sprintf("the last result of %s is not an error", m.Name), repair)
			return false
		}
		expr := exprString(fset, results[0].Type)
		m.Result = strings.TrimPrefix(expr, "*")
		m.ResultPointer = strings.HasPrefix(expr, "*")
		return true
	default:
		c.at(line.Pos, fmt.Sprintf("the method %s returns %d values", m.Name, len(results)), repair)
		return false
	}
}

// namedParam is one parameter with its position.
type namedParam struct {
	Name string
	Type ast.Expr
	Pos  token.Pos
}

// flatten returns one entry for each parameter or result, so that a shared
// type reads as two entries.
func flatten(list *ast.FieldList) []namedParam {
	if list == nil {
		return nil
	}
	var out []namedParam
	for _, f := range list.List {
		if len(f.Names) == 0 {
			out = append(out, namedParam{Type: f.Type, Pos: f.Pos()})
			continue
		}
		for _, name := range f.Names {
			out = append(out, namedParam{Name: name.Name, Type: f.Type, Pos: name.Pos()})
		}
	}
	return out
}

// tagged reports a struct that holds a query tag or a header tag.
func tagged(st *ast.StructType) bool {
	for _, f := range st.Fields.List {
		if f.Tag == nil {
			continue
		}
		tag, err := strconv.Unquote(f.Tag.Value)
		if err != nil {
			continue
		}
		if _, ok := lookupTag(tag, "query"); ok {
			return true
		}
		if _, ok := lookupTag(tag, "header"); ok {
			return true
		}
	}
	return false
}

// pathNames returns the placeholders of a path, in order.
func pathNames(path string) []string {
	var out []string
	rest := path
	for {
		open := strings.Index(rest, "{")
		if open < 0 {
			return out
		}
		end := strings.Index(rest[open:], "}")
		if end < 0 {
			return out
		}
		out = append(out, rest[open+1:open+end])
		rest = rest[open+end+1:]
	}
}

// kindOf returns the kind of a type expression.
func kindOf(expr string) (kind, bool) {
	switch expr {
	case "string":
		return kindString, true
	case "int", "int8", "int16", "int32", "int64", "rune":
		return kindInt, true
	case "uint", "uint8", "uint16", "uint32", "uint64", "byte":
		return kindUint, true
	case "float32", "float64":
		return kindFloat, true
	case "bool":
		return kindBool, true
	default:
		return kindString, false
	}
}

// pair is one key and its value from a directive.
type pair struct {
	key   string
	value string
}

// options reads the key="value" pairs of a directive.
func options(rest string) ([]pair, error) {
	var out []pair
	fields := strings.Fields(rest)
	for _, f := range fields {
		key, value, found := strings.Cut(f, "=")
		if !found {
			return nil, fmt.Errorf("the option %q holds no value", f)
		}
		if !strings.HasPrefix(value, `"`) || !strings.HasSuffix(value, `"`) || len(value) < 2 {
			return nil, fmt.Errorf("the value of %q is not in quotation marks", key)
		}
		out = append(out, pair{key: key, value: value[1 : len(value)-1]})
	}
	return out, nil
}

// exprString returns the source of a type expression.
func exprString(fset *token.FileSet, expr ast.Expr) string {
	var b strings.Builder
	if err := printer.Fprint(&b, fset, expr); err != nil {
		return ""
	}
	return b.String()
}
