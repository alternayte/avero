package assets_test

import "errors"

// errorsAs is the errors.As of the standard library. The tests call it through
// this name, so a test reads in one line.
func errorsAs(err error, target any) bool { return errors.As(err, target) }
