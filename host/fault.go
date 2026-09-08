package host

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// The stages of the run sequence. A fault names the stage that raised it. See
// the SDD, section 5.3.
const (
	StageSetup   = "setup"
	StageMigrate = "migrate"
	StageStart   = "start"
	StageStop    = "stop"
	StageDrain   = "drain"
)

// Fault is one host fault. It names the stage and the subject, states what
// went wrong, and states the repair in one sentence. See DX-7.
type Fault struct {
	// Stage is the step of the run sequence that raised the fault.
	Stage string `json:"stage"`
	// Subject is the component or the thing at fault.
	Subject string `json:"subject"`
	// Message states the fault.
	Message string `json:"message"`
	// Repair states what to do. It is one sentence.
	Repair string `json:"repair"`
	// Err is the cause. It can be nil.
	Err error `json:"-"`
}

// Error returns the message and the repair on two lines.
func (f *Fault) Error() string {
	msg := f.Message
	if f.Err != nil {
		msg += ": " + f.Err.Error()
	}
	return "host: " + msg + "\n  → " + f.Repair
}

// Unwrap returns the cause.
func (f *Fault) Unwrap() error { return f.Err }

// BootFault is the fault of one boot check. See DX-8.
type BootFault struct {
	// Check is the name of the check.
	Check string `json:"check"`
	// Repair states what to do. The check owns the sentence.
	Repair string `json:"repair"`
	// Err is the cause.
	Err error `json:"-"`
}

// Error returns the message and the repair on two lines.
func (f *BootFault) Error() string {
	return "host: the boot check " + f.Check + " failed: " + f.Err.Error() + "\n  → " + f.Repair
}

// Unwrap returns the cause.
func (f *BootFault) Unwrap() error { return f.Err }

// BootFaults holds every boot check that failed. Avero runs every check. It
// does not stop at the first fault.
type BootFaults struct {
	Faults []*BootFault `json:"faults"`
}

// Error returns a count and one entry for each fault.
func (l *BootFaults) Error() string {
	var b strings.Builder
	if len(l.Faults) == 1 {
		b.WriteString("host: 1 boot check failed\n\n")
	} else {
		fmt.Fprintf(&b, "host: %d boot checks failed\n\n", len(l.Faults))
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
func (l *BootFaults) Unwrap() []error {
	out := make([]error, len(l.Faults))
	for i, f := range l.Faults {
		out[i] = f
	}
	return out
}

// Exit writes err to w and returns the process exit code. It returns 0 for a
// nil error and 1 for any fault. The caller owns the call to os.Exit.
func Exit(w io.Writer, err error) int {
	if err == nil {
		return 0
	}
	var boot *BootFaults
	if errors.As(err, &boot) {
		_, _ = io.WriteString(w, boot.Error())
		return 1
	}
	_, _ = fmt.Fprintf(w, "%v\n", err)
	return 1
}
