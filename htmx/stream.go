package htmx

import (
	"bytes"
	"errors"

	"github.com/alternayte/avero/internal/sse"
	"github.com/alternayte/avero/router"
	"github.com/alternayte/avero/view"
)

// ErrClosed states that the client closed the stream. Send returns it after a
// disconnect, so the handler stops and the goroutine ends.
var ErrClosed = sse.ErrClosed

// Stream writes fragments to one client. The htmx SSE extension reads them: an
// element with sse-swap names the event, and the data lines hold the markup.
type Stream struct {
	out *sse.Stream
	ctx *router.Ctx
}

// Done returns a channel that closes when the client disconnects.
func (s *Stream) Done() <-chan struct{} { return s.out.Done() }

// Ctx returns the request that opened the stream.
func (s *Stream) Ctx() *router.Ctx { return s.ctx }

// Send renders a component and writes it as one event.
//
//	<div hx-ext="sse" sse-connect="/rows" sse-swap="row"></div>
func (s *Stream) Send(event string, c view.Component) error {
	var buf bytes.Buffer
	if err := c.Render(s.out.Context(), &buf); err != nil {
		return err
	}
	return s.SendHTML(event, buf.String())
}

// SendHTML writes markup as one event. A line of the markup becomes one data
// line, because a frame cannot carry a new line.
func (s *Stream) SendHTML(event, html string) error {
	return s.out.Send(sse.Event{Name: event, Data: []string{html}})
}

// Open returns a Response that opens a server sent event stream and runs fn.
//
// The handler holds the request until fn returns, so fn must select on Done. A
// disconnect makes every send return ErrClosed, so a loop that ignores Done
// still ends.
func Open(fn func(s *Stream) error) router.Response { return streamResponse{fn: fn} }

// streamResponse opens the stream when the router writes the response.
type streamResponse struct {
	fn func(s *Stream) error
}

// Status returns 200. A stream carries its faults in its frames.
func (r streamResponse) Status() int { return 200 }

// Write opens the stream and runs the function of the handler.
func (r streamResponse) Write(c *router.Ctx) error {
	out, err := sse.New(c.Writer(), c.Request())
	if err != nil {
		return err
	}
	if err := r.fn(&Stream{out: out, ctx: c}); err != nil {
		if errors.Is(err, ErrClosed) {
			// The client left. That is an end, not a fault.
			return nil
		}
		return err
	}
	return nil
}
