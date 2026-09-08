package module

import (
	"fmt"
	"strings"
)

// Fault is one module fault. It names the position in the code that the person
// wrote, states what went wrong, and states the repair in one sentence. See
// DX-6 and DX-7.
type Fault struct {
	// File is the file that holds the fault. It is empty when no position
	// applies.
	File string `json:"file"`
	// Line is the line that holds the fault.
	Line int `json:"line"`
	// Message states the fault.
	Message string `json:"message"`
	// Repair states what to do. It is one sentence.
	Repair string `json:"repair"`
}

// Error returns the position, the message and the repair.
func (f *Fault) Error() string {
	if f.File == "" {
		return fmt.Sprintf("module: %s\n  → %s", f.Message, f.Repair)
	}
	return fmt.Sprintf("module: %s:%d: %s\n  → %s", f.File, f.Line, f.Message, f.Repair)
}

// Faults holds every fault that one inspection found. The module system
// reports every fault, not only the first.
type Faults struct {
	Faults []*Fault `json:"faults"`
}

// Error returns a count and one entry for each fault.
func (l *Faults) Error() string {
	var b strings.Builder
	if len(l.Faults) == 1 {
		b.WriteString("module: 1 fault\n\n")
	} else {
		fmt.Fprintf(&b, "module: %d faults\n\n", len(l.Faults))
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
