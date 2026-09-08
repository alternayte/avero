package view

import (
	"bytes"
	"context"
	"net/http"
	"net/url"

	"github.com/alternayte/avero/router"
)

// ContentType is the content type of a rendered page.
const ContentType = "text/html; charset=utf-8"

// response renders a component.
type response struct {
	code      int
	component Component
}

// Status returns the code that Write sends.
func (r response) Status() int { return r.code }

// Write renders the component into a buffer and then sends it. The buffer
// keeps a render fault away from the client: a fault returns an error and
// writes no byte, so the router answers with the error response.
func (r response) Write(c *router.Ctx) error {
	var buf bytes.Buffer
	if err := r.component.Render(RenderContext(c), &buf); err != nil {
		return err
	}
	c.Header().Set("Content-Type", ContentType)
	c.WriteHeader(r.code)
	_, err := c.Writer().Write(buf.Bytes())
	return err
}

// View renders a component with 200 and the HTML content type.
//
//	return view.View(pages.Index(posts)), nil
func View(c Component) router.Response { return response{code: http.StatusOK, component: c} }

// Status renders a component with this status, for example 404 or 422. The
// root package names it ViewStatus.
func Status(code int, c Component) router.Response {
	return response{code: code, component: c}
}

// RenderContext returns the context that a component reads. It carries the
// toasts of the response, the CSRF token, the submitted form and the
// validation result.
//
// A component therefore needs no Ctx, and a test can build the same context
// with WithOld, WithToasts, WithCSRFToken and WithFields.
func RenderContext(c *router.Ctx) context.Context {
	ctx := c.Context()
	ctx = WithToasts(ctx, c.Toasts())
	ctx = WithCSRFToken(ctx, c.CSRFToken())
	ctx = WithFields(ctx, router.FieldsFrom(ctx))
	return WithOld(ctx, submitted(c))
}

// submitted returns the form values of the request. It parses the body one
// time. The request keeps the result, so a second call costs nothing.
func submitted(c *router.Ctx) url.Values {
	req := c.Request()
	if req.PostForm == nil {
		// A GET request carries no form. ParseForm on it costs one parse of
		// the query, and it returns an error for a body that is not a form.
		_ = req.ParseForm()
	}
	return req.PostForm
}
