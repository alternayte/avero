// Package verify runs the gate of an application and reports one record for
// each step, S16.
//
// `avero verify --json` emits the report, and the MCP tool run_verify returns
// the same document. The order of the records is the order of the gate, so two
// runs on one tree give the same order. See AN-3 and AN-4.
package verify

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

// SchemaID names the schema that a JSON verify report follows. See
// verify/schema.json.
const SchemaID = "https://avero.dev/schema/verify-report/v1"

// Schema holds verify/schema.json. `avero verify --json` emits a document that
// this schema accepts. See AN-3.
//
//go:embed schema.json
var Schema []byte

// The status of one record.
const (
	// StatusPass marks a step that holds.
	StatusPass = "pass"
	// StatusFail marks a step that does not hold.
	StatusFail = "fail"
)

// Record is one step of the gate.
type Record struct {
	// Name names the step, such as gofmt.
	Name string `json:"name"`
	// Status is pass or fail.
	Status string `json:"status"`
	// DurationMS is the time of the step in milliseconds.
	DurationMS int64 `json:"duration_ms"`
	// Message states the fault. It is empty for a step that passes.
	Message string `json:"message"`
	// File names the file that holds the fault, or the empty string.
	File string `json:"file"`
	// Line names the line that holds the fault, or zero.
	Line int `json:"line"`
	// Column names the column that holds the fault, or zero.
	Column int `json:"column"`
}

// Report is the answer of the gate.
type Report struct {
	// OK reports a report with no failed step.
	OK bool `json:"ok"`
	// Records holds one row for each step, in the order of the gate. See
	// AN-4.
	Records []Record `json:"records"`
}

// Step is one command of the gate. A step with no command runs the function
// that Run holds for it, so the generated files are proven in this process and
// the application needs no dependency on the avero binary.
type Step struct {
	// Name names the step.
	Name string
	// Args is the command line, with the program first.
	Args []string
	// Run performs a step that runs no command. It returns the fault, or
	// nil.
	Run func(dir string) error
	// AcceptOutput reports a step that fails when it writes output, although
	// it exits with the code zero. gofmt is such a step.
	AcceptOutput bool
}

// Steps returns the gate of an application, in order. The generate step takes
// the check of the generated files, which the caller supplies, because the
// generators live outside this package.
func Steps(generate func(dir string) error) []Step {
	return []Step{
		{Name: "gofmt", Args: []string{"gofmt", "-l", "."}, AcceptOutput: true},
		{Name: "vet", Args: []string{"go", "vet", "./..."}},
		{Name: "generate", Run: generate},
		{Name: "test", Args: []string{"go", "test", "./...", "-count=1"}},
		{Name: "build", Args: []string{"go", "build", "./..."}},
	}
}

// Run performs the gate in dir and returns the report. It returns no error for
// a failed step, because the report states the failure.
func Run(ctx context.Context, dir string, steps []Step) *Report {
	rep := &Report{OK: true, Records: make([]Record, 0, len(steps))}
	for _, step := range steps {
		rep.Records = append(rep.Records, run(ctx, dir, step))
	}
	for _, record := range rep.Records {
		if record.Status == StatusFail {
			rep.OK = false
		}
	}
	return rep
}

// run performs one step.
func run(ctx context.Context, dir string, step Step) Record {
	start := time.Now()
	record := Record{Name: step.Name, Status: StatusPass}

	switch {
	case step.Run != nil:
		if err := step.Run(dir); err != nil {
			record.Status, record.Message = StatusFail, err.Error()
		}
	case len(step.Args) > 0:
		cmd := exec.CommandContext(ctx, step.Args[0], step.Args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		text := strings.TrimSpace(string(out))
		switch {
		case err != nil:
			record.Status, record.Message = StatusFail, text
			if text == "" {
				record.Message = err.Error()
			}
		case step.AcceptOutput && text != "":
			// gofmt lists the files that need a format and exits zero.
			record.Status = StatusFail
			record.Message = "these files need a format:\n" + text
		}
	}

	if record.Status == StatusFail {
		record.File, record.Line, record.Column = Position(record.Message)
	}
	record.DurationMS = time.Since(start).Milliseconds()
	return record
}

// reportJSON is the wire shape of the report.
type reportJSON struct {
	Schema  string   `json:"schema"`
	OK      bool     `json:"ok"`
	Records []Record `json:"records"`
}

// MarshalJSON emits the report with a stable schema. See AN-3.
func (r *Report) MarshalJSON() ([]byte, error) {
	rows := r.Records
	if rows == nil {
		rows = []Record{}
	}
	return json.Marshal(reportJSON{Schema: SchemaID, OK: r.OK, Records: rows})
}

// String returns the table that `avero verify` prints.
func (r *Report) String() string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "STATUS\tSTEP\tTIME\tPOSITION")
	for _, row := range r.Records {
		position := "-"
		if row.File != "" {
			position = row.File
			if row.Line > 0 {
				position = fmt.Sprintf("%s:%d:%d", row.File, row.Line, row.Column)
			}
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%dms\t%s\n", row.Status, row.Name, row.DurationMS, position)
	}
	_ = w.Flush()
	for _, row := range r.Records {
		if row.Status == StatusFail {
			fmt.Fprintf(&b, "\n%s:\n%s\n", row.Name, row.Message)
		}
	}
	return b.String()
}

// positionPattern reads the file, the line and the column of a Go fault. The
// position can stand after a prefix, because `go vet` writes `vet: ./file.go`.
var positionPattern = regexp.MustCompile(`(?m)(?:^|[\s:])([^\s:]+\.go):(\d+)(?::(\d+))?`)

// Position returns the first position that a message names.
func Position(message string) (string, int, int) {
	m := positionPattern.FindStringSubmatch(message)
	if m == nil {
		return "", 0, 0
	}
	line, _ := strconv.Atoi(m[2])
	column, _ := strconv.Atoi(m[3])
	return strings.TrimPrefix(m[1], "./"), line, column
}
