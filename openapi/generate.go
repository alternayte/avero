package openapi

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Options states one description.
type Options struct {
	// Dir is the root of the application.
	Dir string
	// Title is the name of the application. An empty value takes the name of
	// the module.
	Title string
	// Version is the version of the description. An empty value is 0.1.0.
	Version string
	// Server is the address of the application. An empty value writes none.
	Server string
}

// Generate reads the application and returns its description.
//
// It finds each Routes method of a module, and it reads the input type of each
// handler that the routes name. The tags of the input type give the
// parameters, the body and the rules. See S5 and S14.
func Generate(opts Options) (*Document, error) {
	if opts.Dir == "" {
		opts.Dir = "."
	}
	if opts.Version == "" {
		opts.Version = "0.1.0"
	}
	if opts.Title == "" {
		opts.Title = moduleName(opts.Dir)
	}

	packages, err := read(opts.Dir)
	if err != nil {
		return nil, err
	}

	doc := &Document{
		OpenAPI: Version,
		Info:    Info{Title: opts.Title, Version: opts.Version},
		Paths:   map[string]PathItem{},
	}
	if opts.Server != "" {
		doc.Servers = []Server{{URL: opts.Server}}
	}

	for _, p := range packages {
		module := p.name
		if module == "main" {
			// The composition root registers the routes that no module owns.
			module = "app"
		}
		for _, route := range p.routes {
			handler, ok := p.handlers[route.Handler]
			if !ok {
				// A route that names no typed handler carries no input, so
				// the description holds the route and no parameter.
				add(doc, module, route, method{}, structType{})
				continue
			}
			add(doc, module, route, handler, p.structs[handler.Input])
		}
	}
	if len(doc.Paths) == 0 {
		return nil, fmt.Errorf("avero routes: this application states no route\n  → Register a module with routes in wire.go, then run the command again")
	}
	return doc, nil
}

// add records one route in the document.
func add(doc *Document, module string, r route, h method, in structType) {
	item, ok := doc.Paths[r.Path]
	if !ok {
		item = PathItem{}
		doc.Paths[r.Path] = item
	}

	op := Operation{
		OperationID: operationID(module, r),
		Summary:     h.Summary,
		Tags:        []string{module},
		Responses:   responses(r.Method, in),
	}
	op.Parameters, op.RequestBody = shape(r.Method, in)
	item[strings.ToLower(r.Method)] = op
}

// operationID names one operation. A route that carries a function of its own
// takes the method and the path, because it has no handler to name.
func operationID(module string, r route) string {
	if r.Handler != "" {
		return module + "." + r.Handler
	}
	path := strings.Trim(r.Path, "/")
	path = strings.NewReplacer("/", "_", "{", "", "}", "").Replace(path)
	if path == "" {
		path = "root"
	}
	return module + "." + strings.ToLower(r.Method) + "_" + path
}

// shape returns the parameters and the body of one operation.
func shape(httpMethod string, in structType) ([]Parameter, *RequestBody) {
	var parameters []Parameter
	body := Schema{Type: "object", Properties: map[string]Schema{}}
	closed := false
	body.AdditionalProperties = &closed

	for _, f := range in.fields {
		schema := schemaOf(f)
		switch {
		case f.Path != "":
			parameters = append(parameters, Parameter{
				Name: f.Path, In: "path", Required: true, Schema: schema,
			})
		case f.Query != "":
			parameters = append(parameters, Parameter{
				Name: f.Query, In: "query", Required: f.Required, Schema: schema,
			})
		case f.Header != "":
			parameters = append(parameters, Parameter{
				Name: f.Header, In: "header", Required: f.Required, Schema: schema,
			})
		case f.JSON != "" || f.Form != "":
			name := f.JSON
			if name == "" {
				name = f.Form
			}
			body.Properties[name] = schema
			if f.Required {
				body.Required = append(body.Required, name)
			}
		}
	}
	sort.Strings(body.Required)
	sort.Slice(parameters, func(i, j int) bool {
		if parameters[i].In != parameters[j].In {
			return parameters[i].In < parameters[j].In
		}
		return parameters[i].Name < parameters[j].Name
	})

	if !carriesBody(httpMethod) || len(body.Properties) == 0 {
		return parameters, nil
	}
	return parameters, &RequestBody{
		Required: len(body.Required) > 0,
		Content: map[string]MediaType{
			"application/json": {Schema: body},
		},
	}
}

// carriesBody reports a method that carries a body.
func carriesBody(httpMethod string) bool {
	switch strings.ToUpper(httpMethod) {
	case "POST", "PUT", "PATCH":
		return true
	default:
		return false
	}
}

// responses returns the answers of one route. Avero states the answer of a
// fault, because the router writes it. The answer of the handler carries no
// schema, because Go states it and no tag does.
func responses(httpMethod string, in structType) map[string]Response {
	out := map[string]Response{
		"200": {Description: "the answer of the handler"},
	}
	if strings.EqualFold(httpMethod, "POST") {
		out["201"] = Response{Description: "the answer of a handler that wrote a row"}
	}
	if len(in.fields) > 0 {
		out["400"] = Response{Description: "the request does not bind"}
	}
	if in.validated {
		message := Schema{Type: "object", Properties: map[string]Schema{
			"errors": {Type: "object"},
		}}
		out["422"] = Response{
			Description: "one message for each field that failed validation",
			Content:     map[string]MediaType{"application/json": {Schema: message}},
		}
	}
	out["500"] = Response{Description: "the handler returned an error"}
	return out
}

// schemaOf returns the schema of one field.
func schemaOf(f field) Schema {
	schema := Schema{Type: f.JSONType}
	if f.JSONType == "array" {
		schema.Items = &Schema{Type: "string"}
	}
	for _, r := range f.Rules {
		switch r.Name {
		case "min":
			apply(&schema, r.Arg, true)
		case "max":
			apply(&schema, r.Arg, false)
		case "email":
			schema.Format = "email"
		case "uuid":
			schema.Format = "uuid"
		case "oneof":
			schema.Enum = r.Values
		}
	}
	if f.Format != "" {
		schema.Format = f.Format
	}
	return schema
}

// apply records the bound of a rule on a schema.
func apply(schema *Schema, arg string, lower bool) {
	value, err := strconv.ParseFloat(arg, 64)
	if err != nil {
		return
	}
	if schema.Type == "string" {
		n := int(value)
		if lower {
			schema.MinLength = &n
			return
		}
		schema.MaxLength = &n
		return
	}
	if lower {
		schema.Minimum = &value
		return
	}
	schema.Maximum = &value
}

// moduleName returns the name of the module of the application.
func moduleName(dir string) string {
	body, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return filepath.Base(dir)
	}
	for _, line := range strings.Split(string(body), "\n") {
		if after, found := strings.CutPrefix(strings.TrimSpace(line), "module "); found {
			path := strings.TrimSpace(after)
			return path[strings.LastIndex(path, "/")+1:]
		}
	}
	return filepath.Base(dir)
}

// read parses every package of the application.
func read(dir string) ([]pkg, error) {
	var dirs []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if path != dir && (name == "vendor" || name == "testdata" || name == "node_modules" || strings.HasPrefix(name, ".")) {
			return filepath.SkipDir
		}
		dirs = append(dirs, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("avero routes: %s does not read\n  → Run the command in the directory of an Avero application", dir)
	}
	sort.Strings(dirs)

	var out []pkg
	fset := token.NewFileSet()
	for _, d := range dirs {
		byPackage, err := parseDir(fset, d)
		if err != nil {
			continue
		}
		names := make([]string, 0, len(byPackage))
		for name := range byPackage {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			p := scanPackage(name, byPackage[name])
			if len(p.routes) > 0 {
				out = append(out, p)
			}
		}
	}
	return out, nil
}

// parseDir parses the source files of one directory and groups them by package
// name. It reads the files in name order, so two runs give one order. See
// AN-4.
func parseDir(fset *token.FileSet, dir string) (map[string][]*ast.File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	out := map[string][]*ast.File{}
	for _, name := range names {
		file, parseErr := parser.ParseFile(fset, filepath.Join(dir, name), nil,
			parser.ParseComments|parser.SkipObjectResolution)
		if parseErr != nil {
			continue
		}
		out[file.Name.Name] = append(out[file.Name.Name], file)
	}
	return out, nil
}
