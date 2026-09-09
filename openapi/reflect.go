package openapi

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// inputOf reads the input type of a handler and returns the parameters and the
// body of the request.
//
// A field carries its source in a tag: path, query or header for a parameter,
// json or form for a member of the body. The validate tag states the rules,
// which become the bounds of the schema.
func inputOf(t reflect.Type, schemas map[string]Schema) ([]Parameter, *RequestBody) {
	t = deref(t)
	if t.Kind() != reflect.Struct {
		return nil, nil
	}
	var params []Parameter
	body := Schema{Type: "object", Properties: map[string]Schema{}}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		checks := readValidate(f.Tag.Get("validate"))
		switch {
		case name(f, "path") != "":
			params = append(params, parameterOf(f, "path", name(f, "path"), checks))
		case name(f, "query") != "":
			params = append(params, parameterOf(f, "query", name(f, "query"), checks))
		case name(f, "header") != "":
			params = append(params, parameterOf(f, "header", name(f, "header"), checks))
		default:
			member := name(f, "json")
			if member == "" {
				member = name(f, "form")
			}
			if member == "" {
				continue
			}
			body.Properties[member] = withRules(typeSchema(f.Type, schemas), checks)
			if checks.required() {
				body.Required = append(body.Required, member)
			}
		}
	}
	if len(body.Properties) == 0 {
		return params, nil
	}
	sort.Strings(body.Required)
	return params, &RequestBody{Required: true, Content: map[string]MediaType{
		"application/json": {Schema: body},
	}}
}

// parameterOf builds one parameter of a request.
func parameterOf(f reflect.StructField, in, member string, r validateRules) Parameter {
	return Parameter{
		Name:     member,
		In:       in,
		Required: in == "path" || r.required(),
		Schema:   withRules(Schema{Type: goJSONType(f.Type)}, r),
	}
}

// name returns the name that one tag states, without its options.
func name(f reflect.StructField, key string) string {
	value := f.Tag.Get(key)
	if value == "" || value == "-" {
		return ""
	}
	member, _, _ := strings.Cut(value, ",")
	return member
}

// omitempty reports a field that the answer leaves out when it holds the zero
// value. Every other field stands in the answer, so it is required.
func omitempty(f reflect.StructField) bool {
	value := f.Tag.Get("json")
	_, options, _ := strings.Cut(value, ",")
	for _, option := range strings.Split(options, ",") {
		if option == "omitempty" {
			return true
		}
	}
	return false
}

// typeSchema returns the schema of one type. A struct becomes a reference, and
// the components carry it.
func typeSchema(t reflect.Type, schemas map[string]Schema) Schema {
	t = deref(t)
	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		item := typeSchema(t.Elem(), schemas)
		return Schema{Type: "array", Items: &item}
	case reflect.Map:
		return Schema{Type: "object"}
	case reflect.Struct:
		if t.Name() == "" {
			return Schema{Type: "object"}
		}
		collectInto(schemas, t)
		return Schema{Ref: "#/components/schemas/" + t.Name()}
	default:
		return Schema{Type: goJSONType(t)}
	}
}

// collectInto records the schema of a struct and of every struct that it
// names. The entry stands before the walk, so a type that names itself ends.
func collectInto(schemas map[string]Schema, t reflect.Type) {
	if _, ok := schemas[t.Name()]; ok {
		return
	}
	schemas[t.Name()] = Schema{Type: "object"}

	closed := false
	out := Schema{Type: "object", Properties: map[string]Schema{}, AdditionalProperties: &closed}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		member := name(f, "json")
		if member == "" {
			continue
		}
		out.Properties[member] = typeSchema(f.Type, schemas)
		if !omitempty(f) {
			// Go writes every field of a struct, so the answer carries it.
			out.Required = append(out.Required, member)
		}
	}
	sort.Strings(out.Required)
	schemas[t.Name()] = out
}

// deref returns the type behind a pointer.
func deref(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

// goJSONType returns the JSON type of a Go type.
func goJSONType(t reflect.Type) string {
	switch deref(t).Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Struct, reflect.Map:
		return "object"
	default:
		return "string"
	}
}

// validateRules holds the validate tag of one field.
type validateRules map[string]string

// readValidate parses one validate tag.
func readValidate(tag string) validateRules {
	out := validateRules{}
	for _, part := range strings.Split(tag, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		head, arg, _ := strings.Cut(part, "=")
		out[head] = arg
	}
	return out
}

// required reports the rule that states a value the route needs.
func (r validateRules) required() bool {
	_, ok := r["required"]
	return ok
}

// withRules puts the bounds of the rules on a schema.
func withRules(s Schema, r validateRules) Schema {
	for head, arg := range r {
		switch head {
		case "min":
			bound(&s, arg, true)
		case "max":
			bound(&s, arg, false)
		case "email":
			s.Format = "email"
		case "uuid":
			s.Format = "uuid"
		case "oneof":
			s.Enum = strings.Fields(arg)
		}
	}
	return s
}

// bound puts one bound on a schema. A string carries a length, and a number
// carries a value.
func bound(s *Schema, arg string, low bool) {
	value, err := strconv.ParseFloat(arg, 64)
	if err != nil {
		return
	}
	if s.Type == "string" {
		length := int(value)
		if low {
			s.MinLength = &length
			return
		}
		s.MaxLength = &length
		return
	}
	if low {
		s.Minimum = &value
		return
	}
	s.Maximum = &value
}
