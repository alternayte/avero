// Package budget records a development experience measurement in the
// artifacts of the repository. See the SDD, section 3 and section 7, step 11.
package budget

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Dir is the directory that holds the artifacts.
const Dir = "artifacts"

// The files that the package writes.
const (
	// JSONFile holds the measurements that a machine reads.
	JSONFile = "verification.json"
	// MarkdownFile holds the table that a person reads.
	MarkdownFile = "verification.md"
)

// mu keeps two tests of one package from writing at the same time.
var mu sync.Mutex

// Measurement is one budget and its result.
type Measurement struct {
	// ID is the identifier of the requirement, such as DX-1.
	ID string `json:"id"`
	// Requirement states the requirement in one line.
	Requirement string `json:"requirement"`
	// Budget is the limit in seconds.
	Budget float64 `json:"budget_seconds"`
	// Elapsed is the measurement in seconds.
	Elapsed float64 `json:"elapsed_seconds"`
	// State is pass or fail.
	State string `json:"state"`
	// MeasuredAt is the time of the measurement.
	MeasuredAt string `json:"measured_at"`
}

// Record writes one measurement into the artifacts. It merges with the
// measurements that another run wrote, so one table holds every budget.
func Record(root, id, requirement string, elapsed, limit time.Duration) error {
	mu.Lock()
	defer mu.Unlock()

	dir := filepath.Join(root, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("the artifacts directory does not open: %w", err)
	}
	rows := read(filepath.Join(dir, JSONFile))
	state := "pass"
	if elapsed > limit {
		state = "fail"
	}
	rows[id] = Measurement{
		ID:          id,
		Requirement: requirement,
		Budget:      limit.Seconds(),
		Elapsed:     elapsed.Seconds(),
		State:       state,
		MeasuredAt:  time.Now().UTC().Format(time.RFC3339),
	}
	return write(dir, rows)
}

// read returns the measurements of the file, or an empty set.
func read(name string) map[string]Measurement {
	out := map[string]Measurement{}
	body, err := os.ReadFile(name)
	if err != nil {
		return out
	}
	var doc struct {
		Measurements []Measurement `json:"measurements"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return out
	}
	for _, m := range doc.Measurements {
		out[m.ID] = m
	}
	return out
}

// write puts the measurements into the two files.
func write(dir string, rows map[string]Measurement) error {
	ids := make([]string, 0, len(rows))
	for id := range rows {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	list := make([]Measurement, 0, len(ids))
	for _, id := range ids {
		list = append(list, rows[id])
	}
	body, err := json.MarshalIndent(map[string]any{"measurements": list}, "", "  ")
	if err != nil {
		return fmt.Errorf("the measurements do not encode: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, JSONFile), append(body, '\n'), 0o644); err != nil {
		return fmt.Errorf("%s does not write: %w", JSONFile, err)
	}

	var b strings.Builder
	b.WriteString("# Verification\n\n## Development experience budgets\n\n")
	b.WriteString("| ID | Requirement | Budget | Measurement | State |\n|---|---|---|---|---|\n")
	for _, m := range list {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
			m.ID, m.Requirement, seconds(m.Budget), seconds(m.Elapsed), m.State)
	}
	b.WriteString("\nThe gate writes this file. Run `just budgets` to measure again.\n")
	if err := os.WriteFile(filepath.Join(dir, MarkdownFile), []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("%s does not write: %w", MarkdownFile, err)
	}
	return nil
}

// seconds returns a value in seconds or in milliseconds, whichever a person
// reads more easily.
func seconds(v float64) string {
	if v < 1 {
		return fmt.Sprintf("%.0f ms", v*1000)
	}
	return fmt.Sprintf("%.1f s", v)
}
