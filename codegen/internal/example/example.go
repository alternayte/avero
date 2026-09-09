// Package example is the fixture of the generator. Its generated file is
// committed, so the gate compiles and runs the code that the generator writes.
//
// Every supported type and every rule of the SDD appears here one time.
package example

import (
	"time"

	"github.com/alternayte/avero/router"
)

//go:generate go run github.com/alternayte/avero/internal/cmd/averogen .

// Module holds the handlers of the fixture.
type Module struct{}

// CreateInput covers every source and every rule.
type CreateInput struct {
	// BoardID comes from the path, which wins over every other source.
	BoardID string `path:"board_id" validate:"required,uuid"`
	// Title comes from the form or from the body.
	Title string `json:"title" form:"title" validate:"required,min=3,max=50"`
	// Email covers the email rule.
	Email string `json:"email" form:"email" validate:"required,email"`
	// Role covers the oneof rule.
	Role string `json:"role" form:"role" validate:"oneof=admin member viewer"`
	// Page covers a signed integer and the numeric bounds.
	Page int `query:"page" validate:"min=1,max=100"`
	// Size covers a sized integer.
	Size int64 `query:"size"`
	// Ratio covers a float.
	Ratio float64 `query:"ratio"`
	// Notify covers a boolean. A boolean never carries the required rule.
	Notify bool `json:"notify" form:"notify" query:"notify"`
	// Timeout covers a duration.
	Timeout time.Duration `query:"timeout"`
	// StartsAt covers a time.
	StartsAt time.Time `query:"starts_at" json:"starts_at"`
	// Tags covers a repeated value.
	Tags []string `query:"tags" json:"tags"`
	// Computed binds from nothing. The handler fills it.
	Computed string `avero:"-"`
}

// Check is the custom rule. The generated Validate calls it last, so it reads
// every field that the tags already checked.
func (in *CreateInput) Check(_ *router.Ctx, f *router.Fields) {
	if in.Title == "admin" {
		// A custom rule names the field with the name that the person sent,
		// as the generated rules do. A view then reads the message with the
		// name that it writes in the form. See S10.
		f.Add("title", "must not be a reserved word")
	}
}

// Create is a handler. The generator finds CreateInput from this signature.
func (m *Module) Create(_ *router.Ctx, in CreateInput) (router.Response, error) {
	return router.JSON(201, in), nil
}

// ListInput covers a type with no custom rule.
type ListInput struct {
	// Query comes from the query string only.
	Query string `query:"q" validate:"max=64"`
}

// List is a second handler.
func (m *Module) List(_ *router.Ctx, in ListInput) (router.Response, error) {
	return router.JSON(200, in), nil
}
