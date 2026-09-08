package codegen

// kind classifies the type of a field, so that the emitter writes one
// conversion for it. The generator supports the types that a request can carry
// as text.
type kind int

const (
	kindInvalid kind = iota
	kindString
	kindBool
	kindInt
	kindUint
	kindFloat
	kindDuration
	kindTime
	kindStringSlice
)

// text reports whether a rule that reads characters applies to this kind.
func (k kind) text() bool { return k == kindString }

// number reports whether a rule that compares a value applies to this kind.
func (k kind) number() bool {
	return k == kindInt || k == kindUint || k == kindFloat || k == kindDuration
}

// classify returns the kind of a type expression, written as source text.
func classify(expr string) (kind, bool) {
	switch expr {
	case "string":
		return kindString, true
	case "bool":
		return kindBool, true
	case "int", "int8", "int16", "int32", "int64":
		return kindInt, true
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return kindUint, true
	case "float32", "float64":
		return kindFloat, true
	case "time.Duration":
		return kindDuration, true
	case "time.Time":
		return kindTime, true
	case "[]string":
		return kindStringSlice, true
	}
	return kindInvalid, false
}

// want states the shape that a value must hold, for a bind fault.
func (k kind) want() string {
	switch k {
	case kindBool:
		return "true or false"
	case kindInt, kindUint:
		return "a whole number"
	case kindFloat:
		return "a number"
	case kindDuration:
		return "a duration such as 15s"
	case kindTime:
		return "a time in RFC 3339 form"
	default:
		return "text"
	}
}
