package host

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/alternayte/avero/config"
)

// The names of the doctor report.
const (
	// DoctorSchemaID names the schema that a JSON doctor report follows. See
	// host/doctor_schema.json.
	DoctorSchemaID = "https://avero.dev/schema/doctor-report/v1"
	// StatePass marks a check that holds.
	StatePass = "pass"
	// StateFail marks a check that does not hold.
	StateFail = "fail"
)

// DoctorSchema holds host/doctor_schema.json. `avero doctor --json` emits a
// document that this schema accepts. See AN-3.
//
//go:embed doctor_schema.json
var DoctorSchema []byte

// CheckResult is one row of the doctor report.
type CheckResult struct {
	// Name states what the check proves.
	Name string `json:"name"`
	// State is pass or fail.
	State string `json:"state"`
	// Message states the fault. It is empty for a check that holds.
	Message string `json:"message"`
	// Repair states what to do. It is empty for a check that holds.
	Repair string `json:"repair"`
}

// DoctorReport is the answer of `avero doctor`. It states one row for each
// configuration fault and one row for each boot check. See DX-8.
type DoctorReport struct {
	// OK reports a report with no failed row.
	OK bool `json:"ok"`
	// Checks holds one row for each check, in the order that the
	// application registered them. The configuration rows stand first.
	Checks []CheckResult `json:"checks"`
	// Config lists every configuration field that the loader read.
	Config *config.Report `json:"config,omitempty"`
}

// Doctor runs the boot checks and turns the configuration faults into rows.
//
// The application calls it before it starts a component, and `avero doctor`
// calls it through the binary of the application. A fault therefore appears
// before the process serves. See DX-8.
func Doctor(ctx context.Context, cfgReport *config.Report, cfgErr error, checks []Check) *DoctorReport {
	rep := &DoctorReport{OK: true, Config: cfgReport}

	var faults *config.FaultList
	if errors.As(cfgErr, &faults) {
		if rep.Config == nil {
			rep.Config = faults.Report
		}
		rows := make([]*config.Fault, len(faults.Faults))
		copy(rows, faults.Faults)
		sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
		for _, f := range rows {
			rep.add(CheckResult{
				Name: "the variable " + f.Name, State: StateFail,
				Message: f.Message, Repair: f.Repair,
			})
		}
	} else if cfgErr != nil {
		rep.add(CheckResult{
			Name: "the configuration", State: StateFail,
			Message: cfgErr.Error(),
			Repair:  "Repair the configuration that the message names. Run `avero doctor` again.",
		})
	}

	for _, c := range checks {
		row := CheckResult{Name: c.Name, State: StatePass}
		if err := c.Run(ctx); err != nil {
			row.State, row.Message, row.Repair = StateFail, err.Error(), c.Repair
		}
		rep.add(row)
	}
	return rep
}

// add records one row.
func (r *DoctorReport) add(row CheckResult) {
	if row.State == StateFail {
		r.OK = false
	}
	r.Checks = append(r.Checks, row)
}

// String returns the table that `avero doctor` prints.
func (r *DoctorReport) String() string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "STATE\tCHECK\tMESSAGE")
	for _, row := range r.Checks {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", row.State, row.Name, row.Message)
	}
	_ = w.Flush()
	for _, row := range r.Checks {
		if row.State == StateFail {
			fmt.Fprintf(&b, "\n%s\n  → %s\n", row.Message, row.Repair)
		}
	}
	if r.OK {
		b.WriteString("\nevery check holds\n")
	}
	return b.String()
}

// Write prints the report to w. It writes the JSON document when asJSON is
// true, and the table when it is false.
func (r *DoctorReport) Write(w io.Writer, asJSON bool) error {
	if !asJSON {
		_, err := io.WriteString(w, r.String())
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// doctorJSON is the wire shape of the report.
type doctorJSON struct {
	Schema string         `json:"schema"`
	OK     bool           `json:"ok"`
	Checks []CheckResult  `json:"checks"`
	Config *config.Report `json:"config,omitempty"`
}

// MarshalJSON emits the report with a stable schema. See AN-3.
func (r *DoctorReport) MarshalJSON() ([]byte, error) {
	rows := r.Checks
	if rows == nil {
		rows = []CheckResult{}
	}
	return json.Marshal(doctorJSON{Schema: DoctorSchemaID, OK: r.OK, Checks: rows, Config: r.Config})
}
