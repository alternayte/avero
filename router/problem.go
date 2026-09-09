package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// ProblemContentType is the media type of a problem document. RFC 9457 states
// it.
const ProblemContentType = "application/problem+json"

// Problem is one error of an API, in the shape that RFC 9457 states.
//
// A handler returns it as an error, and the router writes it. One shape covers
// every failure of the service, so a client reads one document and the
// description of the API states one schema.
//
//	if !found {
//	    return nil, avero.NotFound("post", in.ID)
//	}
//
// A Problem wraps a cause. The log reads the cause, and the client does not,
// so a message of the database never reaches a person outside the service.
type Problem struct {
	// Type is a URI that identifies the kind of fault. The default is
	// "about:blank", which states that the status carries the whole meaning.
	Type string `json:"type,omitempty"`
	// Title is a short name of the kind of fault. It does not change between
	// two answers of the same kind.
	Title string `json:"title"`
	// Code is the HTTP status of the answer.
	Code int `json:"status"`
	// Detail explains this one answer. A person reads it.
	Detail string `json:"detail,omitempty"`
	// Instance is a URI of this one answer, such as the path of the request.
	Instance string `json:"instance,omitempty"`
	// Extensions carry the members that this kind of fault adds, such as the
	// field errors of a validation fault. RFC 9457 allows them beside the
	// members above.
	Extensions map[string]any `json:"-"`

	// cause is the error that the handler holds. The log reads it. The client
	// never reads it.
	cause error
}

// Error states the fault for a log and for errors.Is.
func (p *Problem) Error() string {
	if p.Detail != "" {
		return fmt.Sprintf("%d %s: %s", p.Code, p.Title, p.Detail)
	}
	return fmt.Sprintf("%d %s", p.Code, p.Title)
}

// Unwrap returns the cause, so errors.Is and errors.As reach it.
func (p *Problem) Unwrap() error { return p.cause }

// Status returns the HTTP status of the answer.
func (p *Problem) Status() int { return p.Code }

// Wrap returns a copy of the problem that holds the cause. The client reads
// the same document, and the log reads the cause.
func (p *Problem) Wrap(cause error) *Problem {
	out := *p
	out.cause = cause
	return &out
}

// With returns a copy of the problem that carries one more member.
func (p *Problem) With(name string, value any) *Problem {
	out := *p
	out.Extensions = make(map[string]any, len(p.Extensions)+1)
	for k, v := range p.Extensions {
		out.Extensions[k] = v
	}
	out.Extensions[name] = value
	return &out
}

// Explain returns a copy of the problem that carries the detail.
func (p *Problem) Explain(detail string) *Problem {
	out := *p
	out.Detail = detail
	return &out
}

// at returns a copy of the problem that names the path of the request.
func (p *Problem) at(path string) *Problem {
	out := *p
	out.Instance = path
	return &out
}

// Write sends the problem document.
func (p *Problem) Write(c *Ctx) error {
	body, err := p.MarshalJSON()
	if err != nil {
		c.WriteHeader(http.StatusInternalServerError)
		return err
	}
	c.Header().Set("Content-Type", ProblemContentType)
	c.WriteHeader(p.Code)
	_, err = c.Writer().Write(body)
	return err
}

// MarshalJSON writes the members of RFC 9457 and the extensions beside them.
func (p *Problem) MarshalJSON() ([]byte, error) {
	type shape Problem
	body, err := json.Marshal(shape(*p))
	if err != nil {
		return nil, err
	}
	if len(p.Extensions) == 0 {
		return body, nil
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(body, &members); err != nil {
		return nil, err
	}
	for name, value := range p.Extensions {
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		members[name] = raw
	}
	return json.Marshal(members)
}

// NewProblem returns a problem with the standard title of the status.
func NewProblem(code int, detail string) *Problem {
	return &Problem{Type: "about:blank", Title: http.StatusText(code), Code: code, Detail: detail}
}

// The problems that a service answers most. Each one states the status and the
// title, and the caller adds the detail that names the case.
//
// Compare with errors.Is, because each helper returns a new value:
//
//	if errors.Is(err, avero.ErrNotFound) { ... }
var (
	// ErrNotFound states that the path names no thing. 404.
	ErrNotFound = NewProblem(http.StatusNotFound, "")
	// ErrUnauthorized states that the request carries no identity. 401.
	ErrUnauthorized = NewProblem(http.StatusUnauthorized, "")
	// ErrForbidden states that the identity may not do this. 403.
	ErrForbidden = NewProblem(http.StatusForbidden, "")
	// ErrConflict states that the state of the thing refuses the change. 409.
	ErrConflict = NewProblem(http.StatusConflict, "")
	// ErrBadRequest states that the request does not read. 400.
	ErrBadRequest = NewProblem(http.StatusBadRequest, "")
	// ErrInternal states a fault of the service. 500.
	ErrInternal = NewProblem(http.StatusInternalServerError, "")
)

// Is reports a problem of the same status, so errors.Is(err, ErrNotFound)
// answers for every 404 that a handler returns.
func (p *Problem) Is(target error) bool {
	other, ok := target.(*Problem)
	return ok && other.Code == p.Code
}

// NotFound returns the problem of a thing that the table does not hold.
//
//	return nil, avero.NotFound("post", in.ID)
func NotFound(thing, id string) *Problem {
	return NewProblem(http.StatusNotFound, "the "+thing+" "+id+" does not exist")
}

// Conflict returns the problem of a state that refuses the change.
func Conflict(detail string) *Problem { return NewProblem(http.StatusConflict, detail) }

// Unauthorized returns the problem of a request with no identity.
func Unauthorized(detail string) *Problem {
	return NewProblem(http.StatusUnauthorized, detail)
}

// Forbidden returns the problem of an identity that may not do this.
func Forbidden(detail string) *Problem { return NewProblem(http.StatusForbidden, detail) }

// BadRequest returns the problem of a request that does not read.
func BadRequest(detail string) *Problem { return NewProblem(http.StatusBadRequest, detail) }

// ProblemOf returns the problem that an error carries.
//
// It reads a Problem, or an error that wraps one. Every other error is a fault
// of the service, so the answer is 500 and the message stays in the log.
func ProblemOf(err error) *Problem {
	var p *Problem
	if errors.As(err, &p) {
		return p
	}
	return ErrInternal.Wrap(err)
}
