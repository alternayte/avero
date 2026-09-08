package module

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
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
