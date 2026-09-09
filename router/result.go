package router

import "net/http"

// Result is the answer of a typed handler. The type argument names the body,
// so the compiler holds the shape of the answer and the description of the API
// reads it.
//
//	func (m *Module) Show(c *avero.Ctx, in ShowInput) (avero.Result[View], error)
//
// A handler that answers no body writes Result[NoBody].
type Result[T any] struct {
	// Code is the HTTP status of the answer. A zero value means 200, or 204
	// for an empty body.
	Code int
	// Body is the value that the answer carries as JSON.
	Body T
	// Headers carry the answer beside the body.
	Headers map[string]string
}

// NoBody is the body of an answer that carries none.
type NoBody struct{}

// Status returns the HTTP status of the answer.
func (r Result[T]) Status() int {
	if r.Code != 0 {
		return r.Code
	}
	if _, empty := any(r.Body).(NoBody); empty {
		return http.StatusNoContent
	}
	return http.StatusOK
}

// Write sends the answer.
func (r Result[T]) Write(c *Ctx) error {
	for name, value := range r.Headers {
		c.Header().Set(name, value)
	}
	if _, empty := any(r.Body).(NoBody); empty {
		c.WriteHeader(r.Status())
		return nil
	}
	return JSON(r.Status(), r.Body).Write(c)
}

// OK returns 200 with this body.
func OK[T any](body T) Result[T] { return Result[T]{Code: http.StatusOK, Body: body} }

// Created returns 201 with this body.
func Created[T any](body T) Result[T] { return Result[T]{Code: http.StatusCreated, Body: body} }

// Done returns 204 and no body.
func Done() Result[NoBody] { return Result[NoBody]{Code: http.StatusNoContent} }
