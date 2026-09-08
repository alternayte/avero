package htmx

import "fmt"

// Fault is one htmx fault. It states what went wrong and states the repair in
// one sentence. See DX-7.
type Fault struct {
	// Message states the fault.
	Message string `json:"message"`
	// Repair states what to do. It is one sentence.
	Repair string `json:"repair"`
}

// Error returns the message and the repair.
func (f *Fault) Error() string { return fmt.Sprintf("htmx: %s\n  → %s", f.Message, f.Repair) }

// fault returns one fault as an error.
func fault(message, repair string) error { return &Fault{Message: message, Repair: repair} }
