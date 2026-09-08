package view

import (
	"net/http"

	"github.com/alternayte/avero/router"
)

// WithForm renders the form again when validation fails.
//
// The router calls page with the validation result. The answer carries 422,
// and the render context holds the field errors and the submitted values, so
// the component reads them with Error, HasError and Old.
//
//	r := router.New(view.WithForm(func(c *router.Ctx, f *router.Fields) view.Component {
//	    return pages.NewPost()
//	}))
//
// An application that answers JSON needs no option. The default answer is 422
// with a map of field name to message.
func WithForm(page func(c *router.Ctx, f *router.Fields) Component) router.Option {
	return router.WithValidationResponse(func(c *router.Ctx, f *router.Fields) router.Response {
		return Status(http.StatusUnprocessableEntity, page(c, f))
	})
}
