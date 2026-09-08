package router

import (
	"encoding/json"
	"net/http"
)

// Response is the result of a handler. It states its status before it writes,
// so the transaction middleware decides the commit before anything reaches the
// client. See the SDD, S4.
//
// A handler never writes to the ResponseWriter. It returns a Response and the
// router writes it after the transaction commits.
type Response interface {
	// Status is the HTTP status code that Write will send.
	Status() int
	// Write sends the response. The router calls it one time.
	Write(c *Ctx) error
}

// textResponse writes a body with one content type.
type textResponse struct {
	code        int
	contentType string
	body        []byte
}

func (r textResponse) Status() int { return r.code }

func (r textResponse) Write(c *Ctx) error {
	c.Header().Set("Content-Type", r.contentType)
	c.WriteHeader(r.code)
	_, err := c.Writer().Write(r.body)
	return err
}

// Text returns a plain text response.
func Text(code int, body string) Response {
	return textResponse{code: code, contentType: "text/plain; charset=utf-8", body: []byte(body)}
}

// HTML returns an HTML response. S10 renders a templ component with View.
func HTML(code int, body string) Response {
	return textResponse{code: code, contentType: "text/html; charset=utf-8", body: []byte(body)}
}

// Status returns a response that carries the standard text of the code.
func Status(code int) Response {
	return Text(code, http.StatusText(code))
}

// jsonResponse encodes a value as JSON.
type jsonResponse struct {
	code int
	body any
}

func (r jsonResponse) Status() int { return r.code }

func (r jsonResponse) Write(c *Ctx) error {
	c.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.WriteHeader(r.code)
	return json.NewEncoder(c.Writer()).Encode(r.body)
}

// JSON returns a response that encodes body as JSON.
func JSON(code int, body any) Response { return jsonResponse{code: code, body: body} }

// redirectResponse sends a Location header.
type redirectResponse struct {
	code int
	to   string
}

func (r redirectResponse) Status() int { return r.code }

func (r redirectResponse) Write(c *Ctx) error {
	c.Header().Set("Location", r.to)
	c.WriteHeader(r.code)
	return nil
}

// Redirect returns a redirect. Use 303 after a form post and 302 otherwise.
func Redirect(code int, to string) Response { return redirectResponse{code: code, to: to} }

// emptyResponse sends a status and no body.
type emptyResponse struct{ code int }

func (r emptyResponse) Status() int { return r.code }

func (r emptyResponse) Write(c *Ctx) error {
	c.WriteHeader(r.code)
	return nil
}

// NoContent returns 204 with no body.
func NoContent() Response { return emptyResponse{code: http.StatusNoContent} }

// Empty returns a status with no body.
func Empty(code int) Response { return emptyResponse{code: code} }
