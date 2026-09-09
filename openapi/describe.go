package openapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/alternayte/avero/router"
)

// Describe writes the OpenAPI description of the routes of a running
// application.
//
// The application registers each typed route with the type of its input and
// the type of the body of its answer, so this function reads types and no
// comment. A name that does not exist does not compile, and the description
// therefore names no schema that is absent. See the SDD, S13.
//
// The reflection reads the types one time, when the inspection command runs.
// It never runs on a request path. See design rule 2.
func Describe(name, version string, routes []router.Route, api router.API) *Document {
	if api.Title != "" {
		name = api.Title
	}
	if api.Version != "" {
		version = api.Version
	}
	doc := &Document{
		OpenAPI: Version,
		Info: Info{
			Title: name, Version: version,
			Description:    api.Description,
			TermsOfService: api.TermsOfService,
			Contact:        api.Contact,
			License:        api.License,
		},
		Servers:      api.Servers,
		Tags:         api.Tags,
		ExternalDocs: api.ExternalDocs,
		Paths:        map[string]PathItem{},
	}
	if len(api.Require) > 0 {
		schemes := make([]map[string][]string, 0, len(api.Require))
		for _, scheme := range api.Require {
			schemes = append(schemes, map[string][]string{scheme: {}})
		}
		doc.Security = schemes
	}
	schemas := map[string]Schema{}
	for _, route := range routes {
		if route.Op == nil || route.Mounted {
			continue
		}
		path, item := describeRoute(route, schemas)
		if doc.Paths[path] == nil {
			doc.Paths[path] = PathItem{}
		}
		doc.Paths[path][strings.ToLower(route.Method)] = item
	}
	if len(schemas) > 0 || len(api.Security) > 0 {
		doc.Components = &Components{Schemas: schemas, SecuritySchemes: api.Security}
	}
	return doc
}

// describeRoute builds one operation and returns its path.
func describeRoute(route router.Route, schemas map[string]Schema) (string, Operation) {
	op := route.Op
	out := Operation{
		OperationID: opIDOf(route),
		Summary:     op.Summary,
		Description: op.Description,
		Deprecated:  op.Deprecated,
		Tags:        op.Tags,
		Responses:   map[string]Response{},
	}
	if op.Public {
		// An empty list states a route that needs no identity.
		out.Security = &[]map[string][]string{}
	} else if len(op.Security) > 0 {
		schemes := make([]map[string][]string, 0, len(op.Security))
		for _, name := range op.Security {
			schemes = append(schemes, map[string][]string{name: {}})
		}
		out.Security = &schemes
	}
	if op.Input() != nil {
		out.Parameters, out.RequestBody = inputOf(op.Input(), schemas)
	}
	for _, answer := range op.Answers {
		code := strconv.Itoa(answer.Code)
		next := answerResponse(answer, schemas)
		if first, ok := out.Responses[code]; ok {
			// Two answers of one status state that the answer carries one of
			// two shapes.
			next = mergeAnswers(first, next)
		}
		out.Responses[code] = next
	}
	// Every route answers the faults that the router writes. One problem
	// schema covers them, so a client reads one shape. See RFC 9457.
	problemInto(schemas)
	// A route that binds a value can fail to read it, and a rule of a field
	// can refuse it. A route that binds nothing answers neither.
	if out.RequestBody != nil || len(out.Parameters) > 0 {
		out.Responses[strconv.Itoa(http.StatusBadRequest)] = problemResponse("the request does not read")
		out.Responses[strconv.Itoa(http.StatusUnprocessableEntity)] = problemResponse("one field or more failed validation")
	}
	out.Responses[strconv.Itoa(http.StatusInternalServerError)] = problemResponse("the handler returned an error")
	return oapiPath(route.Pattern), out
}

// opIDOf names the operation for a client generator.
//
// The name reads module.Method, such as posts.List, because a generator writes
// the call of a client from it.
func opIDOf(route router.Route) string {
	name := route.Handler
	name = strings.TrimSuffix(name, "-fm")
	name = strings.ReplaceAll(name, "(*Module).", "")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return name
}

// oapiPath returns the pattern with the wildcards that OpenAPI states. The
// router writes /{$} for the root, and OpenAPI writes /.
func oapiPath(pattern string) string {
	pattern = strings.TrimSuffix(pattern, "{$}")
	if pattern == "" {
		return "/"
	}
	if len(pattern) > 1 {
		pattern = strings.TrimSuffix(pattern, "/")
	}
	return pattern
}

// answerResponse builds one answer.
func answerResponse(answer router.Answer, schemas map[string]Schema) Response {
	out := Response{Description: answer.Description}
	if out.Description == "" {
		out.Description = http.StatusText(answer.Code)
	}
	for _, header := range answer.Headers {
		if out.Headers == nil {
			out.Headers = map[string]HeaderObject{}
		}
		out.Headers[header.Name] = HeaderObject{
			Description: header.Description,
			Schema:      Schema{Type: "string"},
		}
	}
	body := answer.Body()
	if body == nil {
		return out
	}
	out.Content = map[string]MediaType{"application/json": {Schema: typeSchema(body, schemas)}}
	return out
}

// mergeAnswers joins two answers of one status into one that states oneOf.
func mergeAnswers(first, second Response) Response {
	out := first
	if second.Description != "" && first.Description != "" && second.Description != first.Description {
		out.Description = first.Description + ", or " + second.Description
	}
	for name, header := range second.Headers {
		if out.Headers == nil {
			out.Headers = map[string]HeaderObject{}
		}
		out.Headers[name] = header
	}
	firstBody, firstOK := first.Content["application/json"]
	secondBody, secondOK := second.Content["application/json"]
	if !firstOK || !secondOK {
		return out
	}
	one := firstBody.Schema
	if len(one.OneOf) > 0 {
		one.OneOf = append(one.OneOf, secondBody.Schema)
		out.Content["application/json"] = MediaType{Schema: one}
		return out
	}
	out.Content = map[string]MediaType{
		"application/json": {Schema: Schema{OneOf: []Schema{one, secondBody.Schema}}},
	}
	return out
}

// ProblemSchema is the name of the schema of an error. RFC 9457 states the
// members.
const ProblemSchema = "Problem"

// problemInto records the schema of an error one time.
func problemInto(schemas map[string]Schema) {
	if _, ok := schemas[ProblemSchema]; ok {
		return
	}
	open := true
	schemas[ProblemSchema] = Schema{
		Type: "object",
		Properties: map[string]Schema{
			"type":     {Type: "string", Format: "uri"},
			"title":    {Type: "string"},
			"status":   {Type: "integer"},
			"detail":   {Type: "string"},
			"instance": {Type: "string"},
		},
		Required: []string{"title", "status"},
		// A problem carries the members that its kind adds, such as the
		// field errors of a validation fault.
		AdditionalProperties: &open,
	}
}

// problemResponse returns an answer that carries a problem document.
func problemResponse(description string) Response {
	return Response{
		Description: description,
		Content: map[string]MediaType{
			router.ProblemContentType: {Schema: Schema{Ref: "#/components/schemas/" + ProblemSchema}},
		},
	}
}
