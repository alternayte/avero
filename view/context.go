package view

import (
	"context"
	"html"
	"io"
	"net/url"

	"github.com/alternayte/avero/router"
)

// The keys of the values that View puts in the render context. A key of a type
// of this package cannot collide with a key of another package.
type (
	oldKey    struct{}
	toastKey  struct{}
	csrfKey   struct{}
	fieldsKey struct{}
)

// WithOld returns a context that carries the submitted form values. View calls
// it. A test calls it to render a component alone.
func WithOld(ctx context.Context, form url.Values) context.Context {
	return context.WithValue(ctx, oldKey{}, form)
}

// WithToasts returns a context that carries the toasts of the response.
func WithToasts(ctx context.Context, toasts []router.Toast) context.Context {
	return context.WithValue(ctx, toastKey{}, toasts)
}

// WithCSRFToken returns a context that carries the CSRF token of the request.
func WithCSRFToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfKey{}, token)
}

// WithFields returns a context that carries the validation result.
func WithFields(ctx context.Context, f *router.Fields) context.Context {
	return context.WithValue(ctx, fieldsKey{}, f)
}

// Old returns the value that the person submitted for this field. It returns
// the empty string for a request that submitted no form, so a first render
// needs no test.
//
//	<input name="title" value={ view.Old(ctx, "title") }>
func Old(ctx context.Context, field string) string {
	form, _ := ctx.Value(oldKey{}).(url.Values)
	if form == nil {
		return ""
	}
	return form.Get(field)
}

// Fields returns the validation result of this request, or nil.
func Fields(ctx context.Context) *router.Fields {
	if f, ok := ctx.Value(fieldsKey{}).(*router.Fields); ok && f != nil {
		return f
	}
	return router.FieldsFrom(ctx)
}

// Error returns the message of this field, or the empty string.
//
//	if view.HasError(ctx, "title") { <span>{ view.Error(ctx, "title") }</span> }
func Error(ctx context.Context, field string) string {
	f := Fields(ctx)
	if f == nil {
		return ""
	}
	return f.Get(field)
}

// HasError reports whether this field failed validation.
func HasError(ctx context.Context, field string) bool {
	f := Fields(ctx)
	return f != nil && f.Has(field)
}

// Toasts returns the messages of this response, in the order that the handler
// added them. The flash middleware carries a toast across a redirect, so a
// message that a handler set before a redirect appears on the next page.
func Toasts(ctx context.Context) []router.Toast {
	toasts, _ := ctx.Value(toastKey{}).([]router.Toast)
	return toasts
}

// CSRFToken returns the token of this request, or the empty string.
func CSRFToken(ctx context.Context) string {
	token, _ := ctx.Value(csrfKey{}).(string)
	return token
}

// CSRF returns the hidden field that an unsafe form must carry. The CSRF
// middleware reads the field. A form that does not carry it returns 419.
//
//	<form method="post">
//	  @view.CSRF()
//	</form>
func CSRF() Component {
	return Func(func(ctx context.Context, w io.Writer) error {
		_, err := io.WriteString(w,
			`<input type="hidden" name="`+router.CSRFFieldName+
				`" value="`+html.EscapeString(CSRFToken(ctx))+`">`)
		return err
	})
}
