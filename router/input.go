package router

import (
	"context"
	"net/http"
)

// Fields collects one message for each field that failed validation. The first
// message of a field stays, because it names the fault that a person repairs
// first.
//
// Generated code writes into it. See the SDD, S5.
type Fields struct {
	order []string
	msgs  map[string]string
}

// Add records a message for a field. A second message for the same field is
// dropped.
func (f *Fields) Add(field, message string) {
	if f.msgs == nil {
		f.msgs = make(map[string]string, 4)
	}
	if _, ok := f.msgs[field]; ok {
		return
	}
	f.msgs[field] = message
	f.order = append(f.order, field)
}

// Has reports whether the field failed.
func (f *Fields) Has(field string) bool {
	_, ok := f.msgs[field]
	return ok
}

// Get returns the message of a field, or the empty string.
func (f *Fields) Get(field string) string { return f.msgs[field] }

// Len returns the number of fields that failed.
func (f *Fields) Len() int { return len(f.order) }

// Names returns the fields that failed, in the order that they failed.
func (f *Fields) Names() []string { return append([]string(nil), f.order...) }

// Map returns a copy of the messages, keyed by field name.
func (f *Fields) Map() map[string]string {
	out := make(map[string]string, len(f.msgs))
	for k, v := range f.msgs {
		out[k] = v
	}
	return out
}

// Binder fills itself from a request. `avero generate` writes it.
type Binder interface {
	Bind(c *Ctx) error
}

// Validator checks itself and records a message for each field that failed.
// `avero generate` writes it.
type Validator interface {
	Validate(c *Ctx, f *Fields)
}

type fieldsKey struct{}

type oldKey struct{}

// FieldsFrom returns the validation result that In put in ctx. It returns nil
// when the request passed validation. S10 renders the form again from it.
func FieldsFrom(ctx context.Context) *Fields {
	f, _ := ctx.Value(fieldsKey{}).(*Fields)
	return f
}

// OldFrom returns the input that failed validation, as a pointer to the input
// type. It returns nil when the request passed. S10 fills the form again from
// it.
func OldFrom(ctx context.Context) any { return ctx.Value(oldKey{}) }

// In adapts a typed handler to the router.
//
// The constraint requires the Bind and Validate methods that `avero generate`
// writes, so a missing or stale generated file is a compile fault at the route
// and never a fault at request time. See AN-2.
//
//	r.Post("/things", avero.In(m.Create))
func In[T any, P interface {
	*T
	Binder
	Validator
}](fn func(c *Ctx, in T) (Response, error),
) Handler {
	return func(c *Ctx) (Response, error) {
		var in T
		p := P(&in)
		if err := p.Bind(c); err != nil {
			// The message can name an internal detail, so it stays in the
			// log. The client reads the standard document of a 400.
			return BadRequest("the request does not read").
				Wrap(err).at(c.Request().URL.Path), nil
		}
		var fields Fields
		p.Validate(c, &fields)
		if fields.Len() > 0 {
			ctx := context.WithValue(c.Context(), fieldsKey{}, &fields)
			ctx = context.WithValue(ctx, oldKey{}, p)
			c.SetContext(ctx)
			return c.validationResponse(&fields), nil
		}
		return fn(c, in)
	}
}

// validationResponse builds the answer to a validation fault.
//
// The default answers 422 as a problem document, and the field messages ride
// in the errors member beside the members of RFC 9457. One error shape covers
// the whole API. WithValidationResponse replaces it, so the SSR shape renders
// the form again.
func (c *Ctx) validationResponse(f *Fields) Response {
	if c.onInvalid != nil {
		return c.onInvalid(c, f)
	}
	return NewProblem(http.StatusUnprocessableEntity, "one field or more failed validation").
		With("errors", f.Map()).
		at(c.Request().URL.Path)
}
