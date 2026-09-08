package router

import (
	"fmt"
	"strings"
)

// Fault is one router fault. It names the position in the code that the person
// wrote, states what went wrong, and states the repair in one sentence. See
// DX-6 and DX-7.
//
// runtime.Caller gives a file and a line. It gives no column, so a router
// fault names a file and a line only.
type Fault struct {
	// File is the file that registered the route.
	File string `json:"file"`
	// Line is the line that registered the route.
	Line int `json:"line"`
	// Message states the fault.
	Message string `json:"message"`
	// Repair states what to do. It is one sentence.
	Repair string `json:"repair"`
}

// Error returns the position, the message and the repair.
func (f *Fault) Error() string {
	return fmt.Sprintf("router: %s:%d: %s\n  → %s", f.File, f.Line, f.Message, f.Repair)
}

// Faults holds every fault that one router found. The router reports every
// fault, not only the first.
type Faults struct {
	Faults []*Fault `json:"faults"`
}

// Error returns a count and one entry for each fault.
func (l *Faults) Error() string {
	var b strings.Builder
	if len(l.Faults) == 1 {
		b.WriteString("router: 1 fault\n\n")
	} else {
		fmt.Fprintf(&b, "router: %d faults\n\n", len(l.Faults))
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
