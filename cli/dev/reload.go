package dev

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// event is one message of the reload channel.
type event struct {
	// Type is css, reload or fault.
	Type string `json:"type"`
	// Href is the new address of the stylesheet of a css event.
	Href string `json:"href,omitempty"`
	// Message states a fault.
	Message string `json:"message,omitempty"`
}

// hub holds the browsers that listen on the reload channel.
type hub struct {
	mu      sync.Mutex
	clients map[chan event]struct{}
}

// newHub builds the hub.
func newHub() *hub { return &hub{clients: map[chan event]struct{}{}} }

// send writes one event to every browser. A browser that does not read is
// skipped, so one slow page never holds the loop.
func (h *hub) send(e event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c <- e:
		default:
		}
	}
}

// Clients returns the number of browsers that listen. A test reads it to know
// that the channel is open before it makes a change.
func (h *hub) Clients() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// add registers one browser.
func (h *hub) add() chan event {
	c := make(chan event, 8)
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	return c
}

// remove drops one browser.
func (h *hub) remove(c chan event) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

// ServeHTTP serves the reload channel as a server sent event stream.
func (h *hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flush, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "the reload channel needs a writer that flushes", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flush.Flush()

	c := h.add()
	defer h.remove(c)

	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-c:
			body, err := json.Marshal(e)
			if err != nil {
				return
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, body); err != nil {
				return
			}
			flush.Flush()
		}
	}
}
