package router

import (
	"net/http"
	"reflect"
)

// Operation is what one route answers. The registration records it, and
// `avero routes --openapi` reads it from the running application.
//
// The types come from the signature of the handler, so the compiler holds
// them. A comment states nothing, and a name that does not exist does not
// build. See the SDD, S13.
type Operation struct {
	// Method is the HTTP method of the route.
	Method string `json:"method"`
	// Pattern is the full pattern of the route.
	Pattern string `json:"pattern"`
	// Summary names the operation for a person.
	Summary string `json:"summary,omitempty"`
	// Description states the operation at length. A summary is one line, and
	// this holds the paragraph that a reader needs.
	Description string `json:"description,omitempty"`
	// Security names the schemes that the route needs. An empty list means
	// that the route follows the API.
	Security []string `json:"security,omitempty"`
	// Public states a route that needs no identity, even when the API states
	// a scheme.
	Public bool `json:"public,omitempty"`
	// Deprecated marks an operation that a caller must leave.
	Deprecated bool `json:"deprecated,omitempty"`
	// Tags group the operations of a description.
	Tags []string `json:"tags,omitempty"`
	// Handler is the name of the function that a person wrote, such as
	// posts.(*Module).List. It names the operation of the description.
	Handler string `json:"handler,omitempty"`
	// Answers holds one entry for each status that the operation writes.
	Answers []Answer `json:"answers,omitempty"`

	// input is the type that the handler binds. A nil value states a handler
	// with no input.
	input reflect.Type
}

// Answer is one status that an operation writes, and the type of its body.
//
// Two answers of one status state that the answer carries one of two shapes,
// which OpenAPI writes as oneOf.
type Answer struct {
	// Code is the HTTP status.
	Code int `json:"code"`
	// Description states the case that produces this answer.
	Description string `json:"description,omitempty"`
	// Headers name the headers that this answer carries.
	Headers []Header `json:"headers,omitempty"`

	// body is the type of the body. A nil value states an answer with none.
	body reflect.Type
}

// Header is one header that an answer carries.
type Header struct {
	// Name is the name of the header, such as Location.
	Name string `json:"name"`
	// Description states what the header holds.
	Description string `json:"description,omitempty"`
}

// Body returns the type of the body of the answer, or nil.
func (a Answer) Body() reflect.Type { return a.body }

// Input returns the type that the operation binds, or nil.
func (o Operation) Input() reflect.Type { return o.input }

// OpOption states one more fact of an operation.
type OpOption func(*Operation)

// Summary names the operation for a person. It is one line.
func Summary(text string) OpOption {
	return func(o *Operation) { o.Summary = text }
}

// Describe states the operation at length, where a summary is one line.
func Describe(text string) OpOption {
	return func(o *Operation) { o.Description = text }
}

// Secured names the schemes that the route needs. The API states the schemes.
func Secured(schemes ...string) OpOption {
	return func(o *Operation) { o.Security = append(o.Security, schemes...) }
}

// Public states a route that needs no identity, even when the API states a
// scheme for every route.
func Public() OpOption {
	return func(o *Operation) { o.Public = true }
}

// AnswerHeader states a header that one answer carries, such as Location on a
// 201.
func AnswerHeader(code int, name, description string) OpOption {
	return func(o *Operation) {
		for i := range o.Answers {
			if o.Answers[i].Code == code {
				o.Answers[i].Headers = append(o.Answers[i].Headers, Header{Name: name, Description: description})
				return
			}
		}
		o.Answers = append(o.Answers, Answer{
			Code: code, Description: http.StatusText(code),
			Headers: []Header{{Name: name, Description: description}},
		})
	}
}

// Deprecated marks an operation that a caller must leave.
func Deprecated() OpOption {
	return func(o *Operation) { o.Deprecated = true }
}

// Tags group the operations of a description.
func Tags(names ...string) OpOption {
	return func(o *Operation) { o.Tags = append(o.Tags, names...) }
}

// Answers states one more status that the operation writes.
//
// The success answer comes from the type of the handler, so this option states
// the other cases:
//
//	avero.Post(r, "/posts", m.Create, avero.Answers[Problem](409, "the title is taken"))
//
// Two calls with one status state an answer that carries one of two shapes.
// The description then writes oneOf.
func Answers[T any](code int, description string) OpOption {
	return func(o *Operation) {
		var body T
		o.Answers = append(o.Answers, Answer{
			Code: code, Description: description,
			body: reflect.TypeOf(&body).Elem(),
		})
	}
}

// AnswersNothing states one more status that the operation writes with no
// body.
func AnswersNothing(code int, description string) OpOption {
	return func(o *Operation) {
		o.Answers = append(o.Answers, Answer{Code: code, Description: description})
	}
}

// Input names the constraint of a type that `avero generate` prepared.
type Input[T any] interface {
	*T
	Binder
	Validator
}

// Register adds one typed route.
//
// A handler returns the thing that it answers, as an ordinary Go function
// does. The type of the input and the type of the answer come from its
// signature, so a change of either is a compile fault. No wrapper surrounds
// the handler, and no comment states the answer.
//
//	avero.Get(r, "/posts/{id}", m.Show)
//
//	func (m *Module) Show(c *avero.Ctx, in ShowInput) (View, error)
//
// The method of the route states the status: a POST answers 201 and every
// other method answers 200. A handler that answers no body returns
// avero.NoBody, and the status is 204. Ctx.Status names another status.
func Register[T any, P Input[T], V any](
	r *Router, method, pattern string,
	fn func(c *Ctx, in T) (V, error),
	opts ...OpOption,
) {
	var in T
	var body V
	// The name of the handler names the operation. The wrapper below is a
	// closure, so the name comes from the function that a person wrote.
	name := handlerName(fn)
	op := Operation{
		Method: method, Pattern: pattern,
		Handler: name,
		Summary: r.summaryOf(name),
		input:   reflect.TypeOf(&in).Elem(),
	}
	if _, empty := any(body).(NoBody); !empty {
		op.Answers = append(op.Answers, Answer{
			Code: successOf(method), Description: "the answer of the handler",
			body: reflect.TypeOf(&body).Elem(),
		})
	} else {
		op.Answers = append(op.Answers, Answer{
			Code: http.StatusNoContent, Description: "the handler answers no body",
		})
	}
	for _, opt := range opts {
		opt(&op)
	}
	handler := In[T, P](func(c *Ctx, v T) (Response, error) {
		out, err := fn(c, v)
		if err != nil {
			return nil, err
		}
		return answerOf(c, method, out), nil
	})
	r.registerOp(method, pattern, handler, &op, 3)
}

// answerOf turns the value of a handler into a response.
//
// A value of NoBody answers the status and no body. Every other value answers
// JSON. Ctx.Status wins over the status of the method.
func answerOf[V any](c *Ctx, method string, out V) Response {
	_, empty := any(out).(NoBody)
	code := c.StatusOf()
	if code == 0 {
		code = successOf(method)
		if empty {
			code = http.StatusNoContent
		}
	}
	if empty {
		return Empty(code)
	}
	return JSON(code, out)
}

// successOf returns the status that a method answers when it succeeds.
func successOf(method string) int {
	if method == http.MethodPost {
		return http.StatusCreated
	}
	return http.StatusOK
}

// Get registers a typed GET route.
func Get[T any, P Input[T], V any](r *Router, pattern string,
	fn func(c *Ctx, in T) (V, error), opts ...OpOption,
) {
	Register[T, P](r, http.MethodGet, pattern, fn, opts...)
}

// Post registers a typed POST route. The success answer is 201.
func Post[T any, P Input[T], V any](r *Router, pattern string,
	fn func(c *Ctx, in T) (V, error), opts ...OpOption,
) {
	Register[T, P](r, http.MethodPost, pattern, fn, opts...)
}

// Put registers a typed PUT route.
func Put[T any, P Input[T], V any](r *Router, pattern string,
	fn func(c *Ctx, in T) (V, error), opts ...OpOption,
) {
	Register[T, P](r, http.MethodPut, pattern, fn, opts...)
}

// Patch registers a typed PATCH route.
func Patch[T any, P Input[T], V any](r *Router, pattern string,
	fn func(c *Ctx, in T) (V, error), opts ...OpOption,
) {
	Register[T, P](r, http.MethodPatch, pattern, fn, opts...)
}

// Delete registers a typed DELETE route.
func Delete[T any, P Input[T], V any](r *Router, pattern string,
	fn func(c *Ctx, in T) (V, error), opts ...OpOption,
) {
	Register[T, P](r, http.MethodDelete, pattern, fn, opts...)
}
