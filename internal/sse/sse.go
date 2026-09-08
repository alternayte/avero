// Package sse writes a server sent event stream. It is internal, because the
// ds package and the htmx package own the shapes that an application writes.
// See the SDD, S12.
package sse

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrClosed states that the client closed the connection. A stream returns it
// after a disconnect, so a handler stops its work and returns.
var ErrClosed = errors.New("sse: the client closed the stream")

// Event is one frame.
type Event struct {
	// Name is the event type. An empty name writes no event line, so the
	// client reads the frame as a message.
	Name string
	// ID is the identifier of the frame. An empty value writes no id line.
	ID string
	// Retry is the reconnection delay. A zero value writes no retry line.
	Retry time.Duration
	// Data holds one data line for each element. A line that holds a new
	// line becomes two lines, because a frame cannot carry one.
	Data []string
}

// Stream writes frames to one client.
type Stream struct {
	w     http.ResponseWriter
	flush http.Flusher
	ctx   context.Context
	err   error
}

// New starts a stream. It writes the headers of the stream and flushes them,
// so the client opens at once.
//
// It returns an error when the writer cannot flush. A writer that buffers the
// whole response cannot stream, and the fault must appear before the handler
// writes a frame.
func New(w http.ResponseWriter, r *http.Request) (*Stream, error) {
	flush, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("sse: the response writer does not flush")
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flush.Flush()
	return &Stream{w: w, flush: flush, ctx: r.Context()}, nil
}

// Done returns a channel that closes when the client disconnects. A handler
// that waits for work selects on it, so no goroutine stays behind.
func (s *Stream) Done() <-chan struct{} { return s.ctx.Done() }

// Context returns the context of the request.
func (s *Stream) Context() context.Context { return s.ctx }

// Err returns the first fault of the stream, or nil.
func (s *Stream) Err() error { return s.err }

// Send writes one frame and flushes it. It returns ErrClosed after the client
// disconnects, and it writes nothing more after a fault.
func (s *Stream) Send(e Event) error {
	if s.err != nil {
		return s.err
	}
	if err := s.ctx.Err(); err != nil {
		s.err = ErrClosed
		return s.err
	}
	var b strings.Builder
	if e.Name != "" {
		b.WriteString("event: " + e.Name + "\n")
	}
	if e.ID != "" {
		b.WriteString("id: " + e.ID + "\n")
	}
	if e.Retry > 0 {
		fmt.Fprintf(&b, "retry: %d\n", e.Retry.Milliseconds())
	}
	for _, line := range e.Data {
		for _, part := range strings.Split(strings.ReplaceAll(line, "\r\n", "\n"), "\n") {
			b.WriteString("data: " + part + "\n")
		}
	}
	b.WriteString("\n")
	if _, err := io.WriteString(s.w, b.String()); err != nil {
		s.err = err
		return err
	}
	s.flush.Flush()
	return nil
}
