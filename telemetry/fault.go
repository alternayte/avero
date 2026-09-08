package telemetry

// Fault is one telemetry fault. It names the environment variable, states what
// went wrong, and states the repair in one sentence. See DX-7.
type Fault struct {
	// Variable is the environment variable at fault.
	Variable string `json:"variable"`
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
	return "telemetry: " + msg + "\n  → " + f.Repair
}

// Unwrap returns the cause.
func (f *Fault) Unwrap() error { return f.Err }
