package config

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// Fault describes one configuration fault. It names the environment variable,
// states what went wrong, and states the repair in one sentence. See DX-7.
type Fault struct {
	// Path is the struct field, qualified by the configuration type.
	Path string `json:"path"`
	// Name is the environment variable.
	Name string `json:"name"`
	// Message states the fault.
	Message string `json:"message"`
	// Repair states what to do. It is one sentence.
	Repair string `json:"repair"`
}

// Error returns the message and the repair on two lines.
func (f *Fault) Error() string {
	return "config: " + f.Message + "\n  → " + f.Repair
}

// FaultList holds every fault that one load found. The loader never stops at
// the first fault.
type FaultList struct {
	Faults []*Fault `json:"faults"`
	// Report lists every field that the loader read before it stopped. It
	// lets `avero doctor` print the whole table beside the faults.
	Report *Report `json:"report,omitempty"`
}

// Error returns a count and one entry for each fault.
func (l *FaultList) Error() string {
	var b strings.Builder
	if len(l.Faults) == 1 {
		b.WriteString("config: 1 fault\n\n")
	} else {
		fmt.Fprintf(&b, "config: %d faults\n\n", len(l.Faults))
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
func (l *FaultList) Unwrap() []error {
	out := make([]error, len(l.Faults))
	for i, f := range l.Faults {
		out[i] = f
	}
	return out
}

func (l *FaultList) add(f *Fault) { l.Faults = append(l.Faults, f) }

func (l *FaultList) err() error {
	if len(l.Faults) == 0 {
		return nil
	}
	return l
}

// Exit writes err to w and returns the process exit code. It returns 0 for a
// nil error and 1 for any fault. The caller owns the call to os.Exit, because
// Avero owns lifecycle in one place.
func Exit(w io.Writer, err error) int {
	if err == nil {
		return 0
	}
	var list *FaultList
	if errors.As(err, &list) {
		_, _ = io.WriteString(w, list.Error())
		return 1
	}
	_, _ = fmt.Fprintf(w, "config: %v\n", err)
	return 1
}
