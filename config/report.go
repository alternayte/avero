package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
)

// ReportSchemaID names the schema that a JSON report follows. See
// config/schema.json.
const ReportSchemaID = "https://avero.dev/schema/config-report/v1"

// ReportSchema holds config/schema.json. `avero doctor --json` emits a
// document that this schema accepts.
//
//go:embed schema.json
var ReportSchema []byte

// Source states where a value came from.
type Source string

const (
	// SourceEnv marks a value that an environment variable supplied.
	SourceEnv Source = "env"
	// SourceDefault marks a value that a default tag supplied.
	SourceDefault Source = "default"
	// SourceAbsent marks a field that no variable and no default supplied.
	SourceAbsent Source = "absent"
)

// Field is one row of a report.
type Field struct {
	// Path is the struct field, such as DB.Host.
	Path string `json:"path"`
	// Name is the environment variable, such as DB_HOST.
	Name string `json:"name"`
	// Type is the Go type of the field, such as int.
	Type string `json:"type"`
	// Value is the value as text. A secret field reads Redacted.
	Value string `json:"value"`
	// Source states where the value came from.
	Source Source `json:"source"`
	// Secret marks a field that the loader redacts.
	Secret bool `json:"secret"`
	// Required marks a field that the loader demands.
	Required bool `json:"required"`
}

// Report lists every field of a configuration struct with its value and its
// source. `avero doctor` prints it. A secret value never appears.
type Report struct {
	// Fields holds one row for each field, in declaration order.
	Fields []Field `json:"fields"`
}

// Field returns the row with this path.
func (r *Report) Field(path string) (Field, bool) {
	for _, f := range r.Fields {
		if f.Path == path {
			return f, true
		}
	}
	return Field{}, false
}

// String returns a table with a header row and one row for each field.
func (r *Report) String() string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "FIELD\tVARIABLE\tVALUE\tSOURCE")
	for _, f := range r.Fields {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", f.Path, f.Name, f.Value, f.Source)
	}
	_ = w.Flush()
	return b.String()
}

// reportJSON is the wire shape of a report. It is a separate type so that
// MarshalJSON does not call itself.
type reportJSON struct {
	Schema string  `json:"schema"`
	Fields []Field `json:"fields"`
}

// MarshalJSON emits the report with a stable schema. See AN-3.
func (r *Report) MarshalJSON() ([]byte, error) {
	fields := r.Fields
	if fields == nil {
		fields = []Field{}
	}
	return json.Marshal(reportJSON{Schema: ReportSchemaID, Fields: fields})
}
