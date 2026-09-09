// Package openapi writes the OpenAPI description of an application, S14.
//
// `avero routes --openapi` reads the routes of each module and the input type
// of each handler, and it writes an OpenAPI 3.1 document. It reads the source
// and runs nothing, so it needs no database and no running application.
//
// The description carries what the code states: the path, the method, the
// parameters, the body and the validation of each field. It carries no answer
// schema, because a handler states its answer in Go and not in a tag. Write
// the answer of a route in the description of its module when a client needs
// it.
package openapi

import (
	"encoding/json"
	"sort"
	"strings"
)

// Version is the version of the OpenAPI specification that the document
// follows.
const Version = "3.1.0"

// Document is one OpenAPI description.
type Document struct {
	// OpenAPI is the version of the specification.
	OpenAPI string `json:"openapi"`
	// Info states the name and the version of the application.
	Info Info `json:"info"`
	// Servers names the addresses of the application.
	Servers []Server `json:"servers,omitempty"`
	// Paths holds one item for each path, in path order. See AN-4.
	Paths map[string]PathItem `json:"paths"`
	// Components holds the schemas that the operations name.
	Components *Components `json:"components,omitempty"`
}

// Components holds the schemas of the description.
type Components struct {
	// Schemas maps the name of a type to its schema.
	Schemas map[string]Schema `json:"schemas"`
}

// Info states the name and the version of the application.
type Info struct {
	// Title is the name of the application.
	Title string `json:"title"`
	// Version is the version of the description.
	Version string `json:"version"`
	// Description states what the application does.
	Description string `json:"description,omitempty"`
}

// Server is one address of the application.
type Server struct {
	// URL is the address.
	URL string `json:"url"`
	// Description states what the address is.
	Description string `json:"description,omitempty"`
}

// PathItem holds the operations of one path.
type PathItem map[string]Operation

// Operation is one route.
type Operation struct {
	// OperationID names the handler, such as posts.Create.
	OperationID string `json:"operationId"`
	// Summary is the first sentence of the comment of the handler.
	Summary string `json:"summary,omitempty"`
	// Tags names the module that owns the route.
	Tags []string `json:"tags,omitempty"`
	// Parameters holds the path parameters, the query parameters and the
	// headers.
	Parameters []Parameter `json:"parameters,omitempty"`
	// RequestBody states the body of the request.
	RequestBody *RequestBody `json:"requestBody,omitempty"`
	// Responses states the answers of the route.
	Responses map[string]Response `json:"responses"`
}

// Parameter is one value that a route reads outside the body.
type Parameter struct {
	// Name is the name that the request carries.
	Name string `json:"name"`
	// In is path, query or header.
	In string `json:"in"`
	// Required states a value that the route needs.
	Required bool `json:"required"`
	// Schema states the type and the rules of the value.
	Schema Schema `json:"schema"`
}

// RequestBody states the body of a request.
type RequestBody struct {
	// Required states a body that the route needs.
	Required bool `json:"required"`
	// Content holds one media type.
	Content map[string]MediaType `json:"content"`
}

// Response is one answer of a route.
type Response struct {
	// Description states the answer in one line.
	Description string `json:"description"`
	// Content holds one media type. An answer with no body holds none.
	Content map[string]MediaType `json:"content,omitempty"`
}

// MediaType states the shape of one body.
type MediaType struct {
	// Schema states the type of the body.
	Schema Schema `json:"schema"`
}

// Schema is one JSON Schema node.
type Schema struct {
	// Type is string, integer, number, boolean, array or object.
	Type string `json:"type,omitempty"`
	// Format states the shape of a string, such as email or uuid.
	Format string `json:"format,omitempty"`
	// MinLength and MaxLength bound a string.
	MinLength *int `json:"minLength,omitempty"`
	MaxLength *int `json:"maxLength,omitempty"`
	// Minimum and Maximum bound a number.
	Minimum *float64 `json:"minimum,omitempty"`
	Maximum *float64 `json:"maximum,omitempty"`
	// Enum lists the values that the rule oneof states.
	Enum []string `json:"enum,omitempty"`
	// Items states the member of an array.
	Items *Schema `json:"items,omitempty"`
	// Properties holds the members of an object.
	Properties map[string]Schema `json:"properties,omitempty"`
	// Required names the members that the object needs.
	Required []string `json:"required,omitempty"`
	// AdditionalProperties states an object that carries other members.
	AdditionalProperties *bool `json:"additionalProperties,omitempty"`
	// Ref names a schema of the components, such as
	// #/components/schemas/Task.
	Ref string `json:"$ref,omitempty"`
}

// MarshalJSON writes the document with its paths in order. See AN-4.
func (d *Document) MarshalJSON() ([]byte, error) {
	type alias Document
	return json.Marshal((*alias)(d))
}

// String returns the document as JSON.
func (d *Document) String() string {
	body, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return ""
	}
	return string(body)
}

// PathNames returns the paths of the document, in order.
func (d *Document) PathNames() []string {
	out := make([]string, 0, len(d.Paths))
	for name := range d.Paths {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Operation returns one operation of the document.
func (d *Document) Operation(method, path string) (Operation, bool) {
	item, ok := d.Paths[path]
	if !ok {
		return Operation{}, false
	}
	op, ok := item[strings.ToLower(method)]
	return op, ok
}
