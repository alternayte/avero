package dev_test

import (
	"bytes"
	"io"
	"net/http"
	"sync"
)

// streamRecorder is a ResponseWriter that a test reads while the handler
// writes. httptest.ResponseRecorder holds its body until the handler returns,
// so a stream needs this one.
type streamRecorder struct {
	header http.Header

	mu     sync.Mutex
	buf    bytes.Buffer
	signal chan struct{}
	closed bool
}

// newStreamRecorder builds the recorder.
func newStreamRecorder() *streamRecorder {
	return &streamRecorder{header: http.Header{}, signal: make(chan struct{}, 1)}
}

// Header returns the header of the answer.
func (r *streamRecorder) Header() http.Header { return r.header }

// WriteHeader records the status.
func (r *streamRecorder) WriteHeader(int) {}

// Write records the bytes and wakes the reader.
func (r *streamRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	n, err := r.buf.Write(p)
	r.mu.Unlock()
	select {
	case r.signal <- struct{}{}:
	default:
	}
	return n, err
}

// Flush answers the Flusher contract of the stream.
func (r *streamRecorder) Flush() {}

// Read returns the bytes that the handler wrote. It blocks until one arrives.
func (r *streamRecorder) Read(p []byte) (int, error) {
	for {
		r.mu.Lock()
		if r.buf.Len() > 0 {
			n, err := r.buf.Read(p)
			r.mu.Unlock()
			return n, err
		}
		closed := r.closed
		r.mu.Unlock()
		if closed {
			return 0, io.EOF
		}
		<-r.signal
	}
}
