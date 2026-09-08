package router

// The levels that a Toast carries.
const (
	ToastInfo    = "info"
	ToastSuccess = "success"
	ToastWarning = "warning"
	ToastError   = "error"
)

// Toast is one message that the next rendered page shows. The flash middleware
// carries a toast across a redirect. See the SDD, S10.
type Toast struct {
	// Level is one of info, success, warning or error.
	Level string `json:"level"`
	// Message is the text that the person reads.
	Message string `json:"message"`
}

// Toast adds a message to this response.
func (c *Ctx) Toast(t Toast) { c.toasts = append(c.toasts, t) }

// Info adds an information message.
func (c *Ctx) Info(message string) { c.Toast(Toast{Level: ToastInfo, Message: message}) }

// Success adds a success message.
func (c *Ctx) Success(message string) { c.Toast(Toast{Level: ToastSuccess, Message: message}) }

// Warning adds a warning message.
func (c *Ctx) Warning(message string) { c.Toast(Toast{Level: ToastWarning, Message: message}) }

// Error adds an error message.
func (c *Ctx) Error(message string) { c.Toast(Toast{Level: ToastError, Message: message}) }

// Toasts returns the messages of this response, in the order that the handler
// added them.
func (c *Ctx) Toasts() []Toast { return c.toasts }
