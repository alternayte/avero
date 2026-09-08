package host

import (
	"net/http"
	"strings"
)

// handleReadyz answers the readiness probe. It returns 200 only when the gate
// is open and every Ready component returns nil. See the SDD, section 5.1.
//
// The body names the components that are not ready. It never carries the
// message of the error, because the endpoint has no authentication.
func (a *App) handleReadyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if !a.open.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("the readiness gate is closed\n"))
		return
	}
	var notReady []string
	for _, c := range a.components {
		probe, ok := c.(Ready)
		if !ok {
			continue
		}
		if err := probe.Ready(r.Context()); err != nil {
			notReady = append(notReady, c.Name())
			a.log.DebugContext(r.Context(), "component is not ready",
				"component", c.Name(), "error", err)
		}
	}
	if len(notReady) > 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("not ready: " + strings.Join(notReady, ", ") + "\n"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready\n"))
}

// mux builds the server handler. The host owns /readyz. The application
// handler owns every other path.
func (a *App) mux() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/readyz", a.handleReadyz)
	if a.handler != nil {
		m.Handle("/", a.handler)
	}
	return m
}
