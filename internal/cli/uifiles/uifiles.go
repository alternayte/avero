// Package uifiles holds the components that `avero ui add` writes into an
// application. The suffix of each file keeps the templ generator of this
// repository away from a template that it must not compile.
package uifiles

import (
	_ "embed"
)

//go:embed toaster.templ.tmpl
var toaster []byte

//go:embed toaster_test.go.tmpl
var toasterTest []byte

// Toaster returns the toaster component of Basecoat. The command writes it to
// internal/ui/toaster.templ of the application.
func Toaster() []byte { return toaster }

// ToasterTest returns the proof of the toaster component. The command writes
// it to toaster_test.go, in the root of the application.
func ToasterTest() []byte { return toasterTest }
