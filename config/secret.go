package config

import (
	"fmt"
	"io"
	"log/slog"
)

// Redacted replaces the value of a secret field wherever Avero prints it.
const Redacted = "********"

// Secret holds a value that must never reach a log line, a trace attribute, an
// error message or the doctor output. Every print verb gives Redacted. Call
// Reveal to read the value.
//
// A field marked `env:"NAME,secret"` is redacted in the report and in every
// fault message whatever its type. Give the field the type Secret to redact it
// in a `%v` format of the configuration struct as well.
type Secret string

// Reveal returns the value. Call it only where the value is used.
func (s Secret) Reveal() string { return string(s) }

// String returns the redaction.
func (s Secret) String() string {
	if s == "" {
		return ""
	}
	return Redacted
}

// Format writes the redaction. It ignores the verb, so no format string can
// print the value.
func (s Secret) Format(f fmt.State, _ rune) {
	_, _ = io.WriteString(f, s.String())
}

// MarshalJSON writes the redaction.
func (s Secret) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", s.String())), nil
}

// MarshalText writes the redaction.
func (s Secret) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// LogValue writes the redaction into a slog record.
func (s Secret) LogValue() slog.Value { return slog.StringValue(s.String()) }
