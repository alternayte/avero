package openapi

import (
	"encoding"
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
	var kinds bodyKinds
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
			if member != "" {
				kinds.json = true
			}
			if form := name(f, "form"); form != "" {
				kinds.form = true
				if member == "" {
					member = form
				}
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
	return params, &RequestBody{Required: true, Content: mediaTypes(kinds, body)}
}

// The media types that a request carries.
const (
	// JSONContent is the media type of a body of JSON.
	JSONContent = "application/json"
	// FormContent is the media type of a body of a form.
	FormContent = "application/x-www-form-urlencoded"
)

// mediaTypes returns the media types that the tags of an input state.
//
// A field with a json tag reads a body of JSON, and a field with a form tag
// reads a form. A page of the ssr shape states both tags, so the route reads
// both, and the description says so.
func mediaTypes(kinds bodyKinds, body Schema) map[string]MediaType {
	out := map[string]MediaType{}
	if kinds.json {
		out[JSONContent] = MediaType{Schema: body}
	}
	if kinds.form {
		out[FormContent] = MediaType{Schema: body}
	}
	if len(out) == 0 {
		out[JSONContent] = MediaType{Schema: body}
	}
	return out
}

// bodyKinds states the tags that the members of a body carry.
type bodyKinds struct {
	json bool
	form bool
}

// parameterOf builds one parameter of a request.
func parameterOf(f reflect.StructField, in, member string, r validateRules) Parameter {
	return Parameter{
		Name:        member,
		In:          in,
		Description: f.Tag.Get("doc"),
		Required:    in == "path" || r.required(),
		Schema:      describeField(f, withRules(Schema{Type: goJSONType(f.Type)}, r)),
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
	if known, ok := knownSchema(t); ok {
		return known
	}
	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			// A slice of bytes reaches JSON as a string of base 64.
			return Schema{Type: "string", Format: "byte"}
		}
		item := typeSchema(t.Elem(), schemas)
		return Schema{Type: "array", Items: &item}
	case reflect.Map:
		// The key of a map reaches JSON as a string, and the value carries
		// its own schema.
		value := typeSchema(t.Elem(), schemas)
		return Schema{Type: "object", AdditionalProperties: &value}
	case reflect.Struct:
		name := schemaName(t)
		if name == "" {
			return Schema{Type: "object"}
		}
		collectInto(schemas, t)
		return Schema{Ref: "#/components/schemas/" + name}
	case reflect.Interface:
		// An empty interface carries any value, so the member states no type.
		return Schema{}
	default:
		return Schema{Type: goJSONType(t)}
	}
}

// knownSchema returns the schema of a type that JSON writes as one value, and
// not as an object of its fields.
//
// time.Time writes a string of RFC 3339, and a type that marshals itself to
// text writes a string. A walk of the fields of such a type would state a
// shape that no answer carries.
func knownSchema(t reflect.Type) (Schema, bool) {
	switch t.String() {
	case "time.Time":
		return Schema{Type: "string", Format: "date-time"}, true
	case "time.Duration":
		return Schema{Type: "integer", Format: "int64"}, true
	case "uuid.UUID":
		return Schema{Type: "string", Format: "uuid"}, true
	case "json.RawMessage":
		return Schema{}, true
	}
	if t.Kind() == reflect.Struct || t.Kind() == reflect.Array {
		if reflect.PointerTo(t).Implements(textMarshaler) || t.Implements(textMarshaler) {
			return Schema{Type: "string"}, true
		}
	}
	return Schema{}, false
}

// textMarshaler is the interface of a type that writes itself as text.
var textMarshaler = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()

// schemaName returns the name of the schema of a struct.
//
// A generic type carries its argument in its name, such as Page[main.Task].
// The characters of such a name do not stand in a reference, so the name loses
// them and reads PageTask.
func schemaName(t reflect.Type) string {
	name := t.Name()
	if name == "" {
		return ""
	}
	if !strings.ContainsAny(name, "[].*") {
		return name
	}
	var b strings.Builder
	for _, r := range name {
		// A reference carries letters and digits, so the name loses the
		// characters that hold the argument of a generic type.
		if strings.ContainsRune("[]*, .", r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// collectInto records the schema of a struct and of every struct that it
// names. The entry stands before the walk, so a type that names itself ends.
func collectInto(schemas map[string]Schema, t reflect.Type) {
	key := schemaName(t)
	if _, ok := schemas[key]; ok {
		return
	}
	schemas[key] = Schema{Type: "object"}

	closed := false
	out := Schema{
		Type: "object", Description: docOf(t),
		Properties: map[string]Schema{}, AdditionalProperties: &closed,
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		member := name(f, "json")
		if member == "" {
			continue
		}
		out.Properties[member] = describeField(f, typeSchema(f.Type, schemas))
		if !omitempty(f) {
			// Go writes every field of a struct, so the answer carries it.
			out.Required = append(out.Required, member)
		}
	}
	sort.Strings(out.Required)
	schemas[key] = out
}

// describeField puts the prose and the values of the tags of a field on its
// schema.
//
// The doc tag states what a member means, and the example tag states one value
// that a reader recognises. A tag holds text and names no type, so a wrong
// word states a wrong sentence and never a reference that does not exist.
//
//	Title string `json:"title" doc:"The name that a person reads" example:"A title"`
func describeField(f reflect.StructField, s Schema) Schema {
	s.Description = f.Tag.Get("doc")
	if value, ok := f.Tag.Lookup("example"); ok {
		s.Example = literal(value, f.Type)
	}
	if value, ok := f.Tag.Lookup("default"); ok {
		s.Default = literal(value, f.Type)
	}
	if value, ok := f.Tag.Lookup("format"); ok {
		s.Format = value
	}
	if value, ok := f.Tag.Lookup("pattern"); ok {
		s.Pattern = value
	}
	for _, option := range strings.Split(f.Tag.Get("openapi"), ",") {
		switch strings.TrimSpace(option) {
		case "readOnly":
			s.ReadOnly = true
		case "writeOnly":
			s.WriteOnly = true
		case "deprecated":
			s.Deprecated = true
		case "uniqueItems":
			s.UniqueItems = true
		}
	}
	// A pointer that the answer always carries reaches JSON as null when it
	// holds nothing. OpenAPI 3.1 states that with a list of types.
	if f.Type.Kind() == reflect.Pointer && !omitempty(f) && s.Ref == "" && s.Type != nil {
		s.Type = []any{s.Type, "null"}
	}
	return s
}

// literal reads the value of a tag as the type of its field, so a number
// reaches the document as a number and not as a string.
func literal(value string, t reflect.Type) any {
	switch goJSONType(t) {
	case "integer":
		if n, err := strconv.ParseInt(value, 10, 64); err == nil {
			return n
		}
	case "number":
		if n, err := strconv.ParseFloat(value, 64); err == nil {
			return n
		}
	case "boolean":
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return value
}

// Documented is a type that states what it means. The description of the API
// carries the sentence.
//
//	func (View) Doc() string { return "One post as the API returns it" }
type Documented interface {
	Doc() string
}

// docOf returns the sentence that a type states, or the empty string.
func docOf(t reflect.Type) string {
	if t.Implements(documented) {
		return reflect.New(t).Elem().Interface().(Documented).Doc()
	}
	if reflect.PointerTo(t).Implements(documented) {
		return reflect.New(t).Interface().(Documented).Doc()
	}
	return ""
}

// documented is the interface of a type that states what it means.
var documented = reflect.TypeOf((*Documented)(nil)).Elem()

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
