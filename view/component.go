// Package view holds the helpers that a server rendered page needs, S10.
//
// The ui package of an application belongs to the scaffold, not to Avero.
// Avero provides only the helpers that a component calls: the old input, the
// field errors, the toasts, the CSRF field, and the Response that renders a
// component.
//
// The package holds no template engine and no dependency on one. Component
// carries the method set of templ.Component, so a templ component satisfies it
// with no adapter and no import. See the SDD, S10.
package view

import (
	"context"
	"io"
)

// Component renders itself. A templ component satisfies it.
type Component interface {
	// Render writes the markup of the component to w.
	Render(ctx context.Context, w io.Writer) error
}

// Func adapts a function to the Component interface. A test and a small
// fragment use it. A page uses templ.
type Func func(ctx context.Context, w io.Writer) error

// Render calls the function.
func (f Func) Render(ctx context.Context, w io.Writer) error { return f(ctx, w) }
