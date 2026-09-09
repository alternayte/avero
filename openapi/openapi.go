// Package openapi writes the OpenAPI description of an application, S13.
//
// `avero routes --openapi` runs the application with its inspection flag and
// reads the operations that the router holds. A route registration touches no
// database and opens no port, so the command needs no infrastructure.
//
// The description carries what the code states: the path, the method, the
// parameters, the body, the validation of each field, and the type of the
// answer. The types come from the signature of each handler, so the compiler
// holds them and no comment can drift from them. Describe reads the types one
// time, and never on a request path. See design rule 2.
package openapi

import (
	"encoding/json"
	"fmt"

	"github.com/alternayte/avero/router"
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
	// Security names the schemes that every route needs.
	Security []map[string][]string `json:"security,omitempty"`
	// Tags describe the groups that the operations name.
	Tags []Tag `json:"tags,omitempty"`
	// ExternalDocs names a document that stands outside this one.
	ExternalDocs *ExternalDocs `json:"externalDocs,omitempty"`
}

// Tag describes one group of operations.
type Tag = router.Tag

// ExternalDocs names a document that stands outside this one.
type ExternalDocs = router.ExternalDocs

// Server is one address that answers.
type Server = router.Server

// SecurityScheme states how a caller proves its identity.
type SecurityScheme = router.SecurityScheme

// Components holds the schemas of the description.
type Components struct {
	// Schemas maps the name of a type to its schema.
	Schemas map[string]Schema `json:"schemas"`
	// SecuritySchemes hold the schemes of the identity, by name.
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
}

// Info states the name and the version of the application.
type Info struct {
	// Title is the name of the application.
	Title string `json:"title"`
	// Version is the version of the description.
	Version string `json:"version"`
	// Description states what the application does.
	Description string `json:"description,omitempty"`
	// TermsOfService is the address of the terms.
	TermsOfService string `json:"termsOfService,omitempty"`
	// Contact names the people who own the API.
	Contact *router.Contact `json:"contact,omitempty"`
	// License names the licence of the API.
	License *router.License `json:"license,omitempty"`
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
	// Description states the operation at length.
	Description string `json:"description,omitempty"`
	// Deprecated marks an operation that a caller must leave.
	Deprecated bool `json:"deprecated,omitempty"`
	// Security names the schemes that the route needs. An empty list states a
	// route that needs no identity.
	Security *[]map[string][]string `json:"security,omitempty"`
}

// Parameter is one value that a route reads outside the body.
type Parameter struct {
	// Name is the name that the request carries.
	Name string `json:"name"`
	// In is path, query or header.
	In string `json:"in"`
	// Description states what the value means.
	Description string `json:"description,omitempty"`
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
	// Headers name the headers that the answer carries.
	Headers map[string]HeaderObject `json:"headers,omitempty"`
}

// HeaderObject is one header of an answer.
type HeaderObject struct {
	// Description states what the header holds.
	Description string `json:"description,omitempty"`
	// Schema states the type of the value.
	Schema Schema `json:"schema"`
}

// MediaType states the shape of one body.
type MediaType struct {
	// Schema states the type of the body.
	Schema Schema `json:"schema"`
}

// Schema is one JSON Schema node.
type Schema struct {
	// Type is string, integer, number, boolean, array or object. A nullable
	// member carries the type and "null", which OpenAPI 3.1 states.
	Type any `json:"type,omitempty"`
	// Title names the schema for a reader.
	Title string `json:"title,omitempty"`
	// Description states what the value means.
	Description string `json:"description,omitempty"`
	// Example is one value that a reader recognises.
	Example any `json:"example,omitempty"`
	// Default is the value that the service uses when the request carries
	// none.
	Default any `json:"default,omitempty"`
	// Pattern is the regular expression that a string must match.
	Pattern string `json:"pattern,omitempty"`
	// MinItems and MaxItems bound an array.
	MinItems *int `json:"minItems,omitempty"`
	MaxItems *int `json:"maxItems,omitempty"`
	// UniqueItems states an array that holds each value one time.
	UniqueItems bool `json:"uniqueItems,omitempty"`
	// ReadOnly states a member that only an answer carries. WriteOnly states
	// a member that only a request carries.
	ReadOnly  bool `json:"readOnly,omitempty"`
	WriteOnly bool `json:"writeOnly,omitempty"`
	// Deprecated marks a member that a caller must leave.
	Deprecated bool `json:"deprecated,omitempty"`
	// OneOf holds the schemas of an answer that carries one of several
	// shapes.
	OneOf []Schema `json:"oneOf,omitempty"`
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
	// AdditionalProperties states an object that carries other members. A
	// bool closes or opens the object, and a schema states the type of the
	// value of a map.
	AdditionalProperties any `json:"additionalProperties,omitempty"`
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

// Parse reads a description that an application wrote.
//
// `avero routes --openapi` runs the application, which prints the document.
// The command reads it back, so it can add the address of a server before it
// writes the file.
func Parse(body []byte) (*Document, error) {
	var doc Document
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("avero routes: the application wrote no description\n  → Run `go build ./...` and repair the fault that the compiler names: %w", err)
	}
	return &doc, nil
}
