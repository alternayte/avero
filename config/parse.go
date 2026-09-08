package config

import (
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var (
	durationType = reflect.TypeOf(time.Duration(0))
	urlType      = reflect.TypeOf(url.URL{})
	stringsType  = reflect.TypeOf([]string(nil))
)

// form describes the shape that a variable must hold. A parse fault names it,
// and the repair sentence gives an example of it.
type form struct {
	// name is the shape, such as "a duration".
	name string
	// example is a value that the parser accepts.
	example string
}

// supported reports whether the loader can fill a field of this type, and
// returns the shape that its value must hold.
func supported(t reflect.Type) (form, bool) {
	switch t {
	case durationType:
		return form{"a duration", "15s"}, true
	case urlType:
		return form{"a URL", "https://host:4318"}, true
	case stringsType:
		return form{"a comma-separated list", "one,two"}, true
	}
	switch t.Kind() {
	case reflect.String:
		return form{"text", "a value"}, true
	case reflect.Bool:
		return form{"a boolean", "true"}, true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return form{"a whole number", "8080"}, true
	default:
		return form{}, false
	}
}

// parse writes raw into dst. It returns false when raw does not hold the shape
// that dst needs. The caller builds the fault, because only the caller knows
// the variable name and whether the field is a secret.
func parse(dst reflect.Value, raw string) bool {
	switch dst.Type() {
	case durationType:
		d, err := time.ParseDuration(raw)
		if err != nil {
			return false
		}
		dst.SetInt(int64(d))
		return true
	case urlType:
		u, err := url.Parse(raw)
		if err != nil {
			return false
		}
		dst.Set(reflect.ValueOf(*u))
		return true
	case stringsType:
		dst.Set(reflect.ValueOf(splitList(raw)))
		return true
	}
	switch dst.Kind() {
	case reflect.String:
		dst.SetString(raw)
		return true
	case reflect.Bool:
		b, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(raw)))
		if err != nil {
			return false
		}
		dst.SetBool(b)
		return true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, dst.Type().Bits())
		if err != nil {
			return false
		}
		dst.SetInt(n)
		return true
	default:
		return false
	}
}

// splitList cuts a comma-separated list and trims each element. An empty
// string gives a list with no element.
func splitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// screamingSnake derives an environment variable name from a field name.
// DatabaseURL gives DATABASE_URL. APIKey gives API_KEY. Write an env tag when
// the derived name is wrong.
func screamingSnake(name string) string {
	runes := []rune(name)
	var b strings.Builder
	for i, r := range runes {
		if i > 0 && isUpper(r) {
			prev := runes[i-1]
			nextIsLower := i+1 < len(runes) && isLower(runes[i+1])
			if !isUpper(prev) || nextIsLower {
				b.WriteByte('_')
			}
		}
		b.WriteRune(toUpper(r))
	}
	return b.String()
}

func isUpper(r rune) bool { return r >= 'A' && r <= 'Z' }
func isLower(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') }
func toUpper(r rune) rune {
	if isLower(r) && r >= 'a' && r <= 'z' {
		return r - 'a' + 'A'
	}
	return r
}
