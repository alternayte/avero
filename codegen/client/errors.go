package client

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Fault is one client fault that no server caused, such as a body that does
// not encode. It states the repair in one sentence. See DX-7.
type Fault struct {
	// Message states the fault.
	Message string `json:"message"`
	// Repair states what to do. It is one sentence.
	Repair string `json:"repair"`
}

// Error returns the message and the repair.
func (f *Fault) Error() string { return fmt.Sprintf("client: %s\n  → %s", f.Message, f.Repair) }

// fault returns one fault as an error.
func fault(message, repair string) error { return &Fault{Message: message, Repair: repair} }

// TransportError states that the request never reached an answer. A retry can
// help, so the policy of the client retries it for an idempotent method.
type TransportError struct {
	// Op names the method of the interface.
	Op string `json:"op"`
	// URL is the address of the call.
	URL string `json:"url"`
	// Err is the fault of the transport.
	Err error `json:"-"`
}

// Error states the call and the fault.
func (e *TransportError) Error() string {
	return fmt.Sprintf("client: %s did not reach %s: %v", e.Op, e.URL, e.Err)
}

// Unwrap returns the fault of the transport, so errors.Is reaches
// context.DeadlineExceeded.
func (e *TransportError) Unwrap() error { return e.Err }

// StatusError states an answer with a status outside the two hundreds.
//
// It keeps the response, so errors.As reaches the header and the body:
//
//	var status *client.StatusError
//	if errors.As(err, &status) && status.Status == http.StatusNotFound {
//	    log.Info("absent", "body", string(status.Body))
//	}
type StatusError struct {
	// Op names the method of the interface.
	Op string `json:"op"`
	// Method is the HTTP method of the call.
	Method string `json:"method"`
	// URL is the address of the call.
	URL string `json:"url"`
	// Status is the status code of the answer.
	Status int `json:"status"`
	// Body holds the answer, up to MaxErrorBody bytes.
	Body []byte `json:"body"`
	// Response is the answer. Its body is already read, and it reads again
	// from the bytes of Body, so a caller can read it one more time.
	Response *http.Response `json:"-"`
}

// Error states the call and the status.
func (e *StatusError) Error() string {
	return fmt.Sprintf("client: %s %s answered %d %s", e.Method, e.URL, e.Status,
		http.StatusText(e.Status))
}

// RetryAfter returns the delay that the server asked for, or zero.
func (e *StatusError) RetryAfter() time.Duration {
	if e.Response == nil {
		return 0
	}
	raw := e.Response.Header.Get("Retry-After")
	if raw == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(raw); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(raw); err == nil {
		if d := time.Until(when); d > 0 {
			return d
		}
	}
	return 0
}

// newStatusError reads the answer and returns the typed error.
func newStatusError(req Request, target *url.URL, res *http.Response) *StatusError {
	body, _ := io.ReadAll(io.LimitReader(res.Body, MaxErrorBody))
	_ = res.Body.Close()
	// The body reads again, so a caller that reaches the response through
	// errors.As can read it.
	res.Body = io.NopCloser(bytes.NewReader(body))
	return &StatusError{
		Op:       req.Op,
		Method:   req.Method,
		URL:      target.String(),
		Status:   res.StatusCode,
		Body:     body,
		Response: res,
	}
}
