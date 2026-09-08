package router

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
)

// ReportSchemaID names the schema that a JSON routes report follows. See
// router/schema.json.
const ReportSchemaID = "https://avero.dev/schema/routes-report/v1"

// ReportSchema holds router/schema.json. `avero routes --json` emits a
// document that this schema accepts. See AN-3.
//
//go:embed schema.json
var ReportSchema []byte

// Report lists every route of a router. `avero routes` prints it.
type Report struct {
	// Routes holds one row for each route, ordered by pattern and then by
	// method, so that two runs give the same order. See AN-4.
	Routes []Route `json:"routes"`
}

// Report returns the routes of the router. It returns every registration fault
// and no report, so a fault stops the process before it serves.
func (r *Router) Report() (*Report, error) {
	if len(r.reg.faults) > 0 {
		return nil, &Faults{Faults: r.reg.faults}
	}
	rows := make([]Route, len(r.reg.routes))
	copy(rows, r.reg.routes)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Pattern != rows[j].Pattern {
			return rows[i].Pattern < rows[j].Pattern
		}
		return rows[i].Method < rows[j].Method
	})
	return &Report{Routes: rows}, nil
}

// Route returns the row with this method and pattern.
func (rep *Report) Route(method, pattern string) (Route, bool) {
	for _, row := range rep.Routes {
		if row.Method == method && row.Pattern == pattern {
			return row, true
		}
	}
	return Route{}, false
}

// String returns a table with a header row and one row for each route.
func (rep *Report) String() string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "METHOD\tPATTERN\tHANDLER\tMIDDLEWARE")
	for _, row := range rep.Routes {
		handler := row.Handler
		if row.Mounted {
			handler += " (mounted)"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			row.Method, row.Pattern, handler, strings.Join(row.Middleware, " → "))
	}
	_ = w.Flush()
	return b.String()
}

// reportJSON is the wire shape of a report. It is a separate type so that
// MarshalJSON does not call itself.
type reportJSON struct {
	Schema string  `json:"schema"`
	Routes []Route `json:"routes"`
}

// MarshalJSON emits the report with a stable schema. See AN-3.
func (rep *Report) MarshalJSON() ([]byte, error) {
	rows := rep.Routes
	if rows == nil {
		rows = []Route{}
	}
	for i := range rows {
		if rows[i].Middleware == nil {
			rows[i].Middleware = []string{}
		}
	}
	return json.Marshal(reportJSON{Schema: ReportSchemaID, Routes: rows})
}
