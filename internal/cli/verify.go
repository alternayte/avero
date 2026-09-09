package cli

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"

	"github.com/alternayte/avero/codegen"
	"github.com/alternayte/avero/codegen/client"
	"strconv"
	"strings"
	"text/tabwriter"
)

// VerifySchemaID names the schema that a JSON verify report follows. See
// internal/cli/verify_schema.json.
const VerifySchemaID = "https://avero.dev/schema/verify-report/v1"

// VerifySchema holds the schema of the verify report. `avero verify --json`
// emits a document that this schema accepts. See AN-3 and S16.
//
//go:embed verify_schema.json
var VerifySchema []byte

// Record is one step of the gate.
type Record struct {
	// Step names the step, such as gofmt.
	Step string `json:"step"`
	// State is pass or fail.
	State string `json:"state"`
	// Message states the fault. It is empty for a step that passes.
	Message string `json:"message"`
	// File names the file that holds the fault, or the empty string.
	File string `json:"file"`
	// Line names the line that holds the fault, or zero.
	Line int `json:"line"`
	// Column names the column that holds the fault, or zero.
	Column int `json:"column"`
}

// VerifyReport is the answer of `avero verify`.
type VerifyReport struct {
	// OK reports a report with no failed step.
	OK bool `json:"ok"`
	// Records holds one row for each step, in the order of the gate. See
	// AN-4.
	Records []Record `json:"records"`
}

// verifyJSON is the wire shape of the report.
type verifyJSON struct {
	Schema  string   `json:"schema"`
	OK      bool     `json:"ok"`
	Records []Record `json:"records"`
}

// MarshalJSON emits the report with a stable schema.
func (r *VerifyReport) MarshalJSON() ([]byte, error) {
	rows := r.Records
	if rows == nil {
		rows = []Record{}
	}
	return json.Marshal(verifyJSON{Schema: VerifySchemaID, OK: r.OK, Records: rows})
}

// String returns the table that `avero verify` prints.
func (r *VerifyReport) String() string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "STATE\tSTEP\tPOSITION")
	for _, row := range r.Records {
		position := "-"
		if row.File != "" {
			position = row.File
			if row.Line > 0 {
				position = fmt.Sprintf("%s:%d:%d", row.File, row.Line, row.Column)
			}
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", row.State, row.Step, position)
	}
	_ = w.Flush()
	for _, row := range r.Records {
		if row.State == stateFail {
			fmt.Fprintf(&b, "\n%s:\n%s\n", row.Step, row.Message)
		}
	}
	return b.String()
}

// The states of one record.
const (
	statePass = "pass"
	stateFail = "fail"
)

// step is one command of the gate.
type step struct {
	// Name states the step.
	Name string
	// Args is the command line, with the program first.
	Args []string
}

// runVerify runs the gate of the application.
func runVerify(ctx context.Context, s Streams, args []string) int {
	asJSON := false
	for _, a := range args {
		switch a {
		case "--json", "-json":
			asJSON = true
		default:
			return failf(s, "avero verify: the flag %q is not known\n  → Run the command with --json or with no flag", a)
		}
	}
	dir := dirOf(s)
	steps := []step{
		{Name: "gofmt", Args: []string{"gofmt", "-l", "."}},
		{Name: "vet", Args: []string{"go", "vet", "./..."}},
		// The generate step runs in this process. The application therefore
		// needs no dependency on the avero binary to prove its generated
		// files.
		{Name: "generate"},
		{Name: "test", Args: []string{"go", "test", "./...", "-count=1"}},
		{Name: "build", Args: []string{"go", "build", "./..."}},
	}

	rep := &VerifyReport{OK: true}
	for _, st := range steps {
		record := Record{Step: st.Name, State: statePass}
		switch st.Name {
		case "generate":
			if err := generateCheck(dir); err != nil {
				record.State, record.Message = stateFail, err.Error()
			}
		default:
			cmd := exec.CommandContext(ctx, st.Args[0], st.Args[1:]...)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			text := strings.TrimSpace(string(out))
			switch {
			case err != nil:
				record.State, record.Message = stateFail, text
				if text == "" {
					record.Message = err.Error()
				}
			case st.Name == "gofmt" && text != "":
				// gofmt lists the files that need a format and exits 0.
				record.State = stateFail
				record.Message = "these files need a format:\n" + text
			}
		}
		if record.State == stateFail {
			record.File, record.Line, record.Column = position(record.Message)
			rep.OK = false
		}
		rep.Records = append(rep.Records, record)
	}

	if asJSON {
		enc := json.NewEncoder(s.Out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return fail(s, err)
		}
	} else {
		_, _ = fmt.Fprint(s.Out, rep.String())
	}
	if !rep.OK {
		return 1
	}
	return 0
}

// generateCheck proves that every generated file of the application is
// current.
func generateCheck(dir string) error {
	if err := codegen.Check(dir); err != nil {
		return err
	}
	return client.Check(dir)
}

// positionPattern reads the file, the line and the column of a Go fault.
var positionPattern = regexp.MustCompile(`(?m)^([^\s:]+\.go):(\d+)(?::(\d+))?`)

// position returns the first position that a message names.
func position(message string) (string, int, int) {
	m := positionPattern.FindStringSubmatch(message)
	if m == nil {
		return "", 0, 0
	}
	line, _ := strconv.Atoi(m[2])
	column, _ := strconv.Atoi(m[3])
	return m[1], line, column
}
