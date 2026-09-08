package router

import (
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
)

// The helpers that generated binding and validation code calls. They keep the
// generated file short enough for a person to read. See design rule 4.

// HasJSONBody reports whether the request carries a JSON body to decode.
func HasJSONBody(r *http.Request) bool {
	if r.Body == nil || r.ContentLength == 0 {
		return false
	}
	ct := r.Header.Get("Content-Type")
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	ct = strings.TrimSpace(ct)
	return ct == "application/json" || strings.HasSuffix(ct, "+json")
}

// Value returns one member of a query string or a form, and reports whether
// it is present.
//
// Generated code parses the query one time and passes the result here, because
// url.URL.Query parses the whole string on each call.
func Value(values url.Values, name string) (string, bool) {
	v, ok := values[name]
	if !ok || len(v) == 0 {
		return "", ok
	}
	return v[0], true
}

// Values returns every member of a repeated query parameter or form field.
func Values(values url.Values, name string) ([]string, bool) {
	v, ok := values[name]
	return v, ok
}

// PathValue returns a path wildcard and reports whether it is present.
func PathValue(r *http.Request, name string) (string, bool) {
	v := r.PathValue(name)
	return v, v != ""
}

// BindFault reports a value that does not hold the shape that a field needs.
// It names the parameter and the shape, and it never carries the value, which
// can be a secret.
type BindFault struct {
	// Parameter is the name of the path, query, form or body member.
	Parameter string
	// Want states the shape, such as "a whole number".
	Want string
}

// Error states the fault.
func (e *BindFault) Error() string {
	return fmt.Sprintf("router: %s must be %s", e.Parameter, e.Want)
}

// NewBindFault builds a bind fault.
func NewBindFault(parameter, want string) error {
	return &BindFault{Parameter: parameter, Want: want}
}

// IsEmail reports whether s is one address. net/mail owns the grammar, so
// Avero adds no dependency and no pattern of its own.
func IsEmail(s string) bool {
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return false
	}
	// ParseAddress accepts a display name. A form field must hold the address
	// alone.
	return addr.Address == s
}

// IsUUID reports whether s is a UUID in the 8-4-4-4-12 form.
func IsUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}

// OneOf reports whether s is one of the values.
func OneOf(s string, values ...string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}
