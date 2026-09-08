package codegen

import (
	"fmt"
	"go/token"
	"strings"
)

// Fault is one generation fault. It names a file, a line and a column in code
// that the person wrote, and it states the repair in one sentence. It never
// points into a template, into a generated file or into Avero. See DX-6 and
// DX-7.
type Fault struct {
	// File is the file that holds the fault.
	File string `json:"file"`
	// Line is the line of the struct field.
	Line int `json:"line"`
	// Column is the column of the struct field.
	Column int `json:"column"`
	// Message states the fault.
	Message string `json:"message"`
	// Repair states what to do. It is one sentence.
	Repair string `json:"repair"`
}

// Error prints the position, the message and the repair.
func (f *Fault) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s\n  → %s", f.File, f.Line, f.Column, f.Message, f.Repair)
}

// Faults holds every fault of one run. The generator reports every fault, not
// only the first.
type Faults struct {
	Faults []*Fault `json:"faults"`
}

// Error returns a count and one entry for each fault.
func (l *Faults) Error() string {
	var b strings.Builder
	if len(l.Faults) == 1 {
		b.WriteString("avero generate: 1 fault\n\n")
	} else {
		fmt.Fprintf(&b, "avero generate: %d faults\n\n", len(l.Faults))
	}
	for i, f := range l.Faults {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(f.Error())
		b.WriteString("\n")
	}
	return b.String()
}

// Unwrap returns each fault so that errors.As reaches a single one.
func (l *Faults) Unwrap() []error {
	out := make([]error, len(l.Faults))
	for i, f := range l.Faults {
		out[i] = f
	}
	return out
}

// collector gathers the faults of one run.
type collector struct {
	fset   *token.FileSet
	faults []*Fault
}

// at records a fault at the position of a struct field.
func (c *collector) at(pos token.Pos, message, repair string) {
	p := c.fset.Position(pos)
	c.faults = append(c.faults, &Fault{
		File: p.Filename, Line: p.Line, Column: p.Column,
		Message: message, Repair: repair,
	})
}

// err returns the faults, or nil.
func (c *collector) err() error {
	if len(c.faults) == 0 {
		return nil
	}
	return &Faults{Faults: c.faults}
}
