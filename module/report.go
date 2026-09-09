package module

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

// ReportSchemaID names the schema that a JSON module report follows. See
// module/schema.json.
const ReportSchemaID = "https://avero.dev/schema/modules-report/v1"

// ReportSchema holds module/schema.json. `avero modules --json` emits a
// document that this schema accepts. See AN-3.
//
//go:embed schema.json
var ReportSchema []byte

// Report lists the contribution of every module. `avero modules` prints it.
type Report struct {
	// Modules holds one row for each module, in registration order, so that
	// two runs give the same order. See AN-4.
	Modules []Contribution `json:"modules"`
}

// Report returns the contribution table. It returns every fault and no report,
// so a fault stops the process before it serves.
func (s *Set) Report() (*Report, error) {
	if err := s.Err(); err != nil {
		return nil, err
	}
	rows := make([]Contribution, len(s.rows))
	copy(rows, s.rows)
	return &Report{Modules: rows}, nil
}

// Module returns the row of this module.
func (rep *Report) Module(name string) (Contribution, bool) {
	for _, row := range rep.Modules {
		if row.Module == name {
			return row, true
		}
	}
	return Contribution{}, false
}

// String returns the contribution table with a header row and one row for each
// module.
func (rep *Report) String() string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "MODULE\tINTERFACES\tROUTES\tINBOX\tPROJECTIONS\tJOBS\tMIGRATIONS")
	for _, row := range rep.Modules {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
			row.Module,
			dash(strings.Join(row.Interfaces, " ")),
			len(row.Routes),
			dash(strings.Join(row.InboxHandlers, " ")),
			dash(strings.Join(row.Projections, " ")),
			dash(strings.Join(row.Jobs, " ")),
			strconv.FormatBool(row.Migrations))
	}
	_ = w.Flush()
	return b.String()
}

// dash returns a dash for an empty cell.
func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// reportJSON is the wire shape of a report. It is a separate type so that
// MarshalJSON does not call itself.
type reportJSON struct {
	Schema  string         `json:"schema"`
	Modules []Contribution `json:"modules"`
}

// MarshalJSON emits the report with a stable schema. Every member holds an
// array, so a reader needs no null test. See AN-3 and AN-4.
func (rep *Report) MarshalJSON() ([]byte, error) {
	rows := make([]Contribution, len(rep.Modules))
	copy(rows, rep.Modules)
	for i := range rows {
		if rows[i].Interfaces == nil {
			rows[i].Interfaces = []string{}
		}
		if rows[i].Routes == nil {
			rows[i].Routes = []RouteDesc{}
		}
		if rows[i].InboxHandlers == nil {
			rows[i].InboxHandlers = []string{}
		}
		if rows[i].Projections == nil {
			rows[i].Projections = []string{}
		}
		if rows[i].Jobs == nil {
			rows[i].Jobs = []string{}
		}
		if rows[i].Description == nil {
			rows[i].Description = &Description{Name: rows[i].Module}
		}
		d := *rows[i].Description
		d.normalise()
		rows[i].Description = &d
	}
	return json.Marshal(reportJSON{Schema: ReportSchemaID, Modules: rows})
}

// WriteReport writes the report to w. It writes the JSON document when asJSON
// is true, and the table when it is false. It returns the faults and writes
// nothing when the inspection found one. `avero modules` calls it. See S14.
func (s *Set) WriteReport(w io.Writer, asJSON bool) error {
	rep, err := s.Report()
	if err != nil {
		return err
	}
	if !asJSON {
		_, err = io.WriteString(w, rep.String())
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

// SchemaSchemaID names the schema that a JSON model report follows. See
// module/schema_models.json.
const SchemaSchemaID = "https://avero.dev/schema/model-report/v1"

// ModelSchema holds module/schema_models.json. `avero schema --json` emits a
// document that this schema accepts. See AN-3.
//
//go:embed schema_models.json
var ModelSchema []byte

// SchemaReport lists the models of every module. `avero schema` prints it.
type SchemaReport struct {
	// Models holds one row for each model, ordered by module and then by
	// model name, so that two runs give the same order. See AN-4.
	Models []ModelRow `json:"models"`
}

// ModelRow is one model of one module.
type ModelRow struct {
	// Module is the module that owns the model.
	Module string `json:"module"`
	// Name is the name of the Go type.
	Name string `json:"name"`
	// Table is the name of the database table.
	Table string `json:"table"`
	// Fields lists the fields of the model.
	Fields []FieldDesc `json:"fields"`
}

// Schema returns the models that the modules describe.
func Schema(s *Set) *SchemaReport {
	out := &SchemaReport{Models: []ModelRow{}}
	for _, row := range s.rows {
		if row.Description == nil {
			continue
		}
		for _, m := range row.Description.Models {
			fields := m.Fields
			if fields == nil {
				fields = []FieldDesc{}
			}
			out.Models = append(out.Models, ModelRow{
				Module: row.Module, Name: m.Name, Table: m.Table, Fields: fields,
			})
		}
	}
	sort.Slice(out.Models, func(i, j int) bool {
		if out.Models[i].Module != out.Models[j].Module {
			return out.Models[i].Module < out.Models[j].Module
		}
		return out.Models[i].Name < out.Models[j].Name
	})
	return out
}

// String returns a table with one row for each model.
func (rep *SchemaReport) String() string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "MODULE\tMODEL\tTABLE\tFIELDS")
	for _, row := range rep.Models {
		names := make([]string, 0, len(row.Fields))
		for _, f := range row.Fields {
			names = append(names, f.Name+" "+f.Type)
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", row.Module, row.Name, row.Table, dash(strings.Join(names, ", ")))
	}
	_ = w.Flush()
	return b.String()
}

// schemaJSON is the wire shape of a model report.
type schemaJSON struct {
	Schema string     `json:"schema"`
	Models []ModelRow `json:"models"`
}

// MarshalJSON emits the report with a stable schema. See AN-3.
func (rep *SchemaReport) MarshalJSON() ([]byte, error) {
	rows := rep.Models
	if rows == nil {
		rows = []ModelRow{}
	}
	return json.Marshal(schemaJSON{Schema: SchemaSchemaID, Models: rows})
}
