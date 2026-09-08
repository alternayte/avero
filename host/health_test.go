package host_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/avero/host"
)

func TestHealthzReturns200BeforeTheGateOpens(t *testing.T) {
	rec := &recorder{}
	slow := newFake(rec, "slow")
	slow.startBlocks = make(chan struct{})
	app := newApp(t, baseConfig(2*time.Second), host.WithComponents(slow))

	stop := run(t, app)
	waitServing(t, app)

	// The process lives although it is not yet in rotation. A liveness probe
	// must not restart it during a slow start.
	if code, _ := get(t, app, "/healthz"); code != http.StatusOK {
		t.Fatalf("/healthz returned %d before the gate opened, want 200", code)
	}
	if code, _ := get(t, app, "/readyz"); code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz returned %d before the gate opened, want 503", code)
	}
	close(slow.startBlocks)
	waitReady(t, app)
	if err := stop(); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
}

func TestHealthzReturns200WhenAReadyComponentFails(t *testing.T) {
	rec := &recorder{}
	r := newReadyFake(rec, "broker")
	app := newApp(t, baseConfig(2*time.Second), host.WithComponents(r))

	stop := run(t, app)
	waitReady(t, app)
	r.setReady(errFake)

	if code, _ := get(t, app, "/healthz"); code != http.StatusOK {
		t.Fatalf("/healthz returned %d, want 200", code)
	}
	if code, _ := get(t, app, "/readyz"); code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz returned %d, want 503", code)
	}
	r.setReady(nil)
	if err := stop(); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
}

func TestHealthzReturns200DuringTheDrain(t *testing.T) {
	rec := &recorder{}
	held := make(chan struct{})
	reached := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/hold", func(w http.ResponseWriter, _ *http.Request) {
		close(reached)
		<-held
		_, _ = w.Write([]byte("done"))
	})
	app := newApp(t, baseConfig(10*time.Second),
		host.WithComponents(newFake(rec, "a")), host.WithHandler(mux))

	stop := run(t, app)
	waitReady(t, app)

	inFlight := make(chan struct{})
	go func() {
		defer close(inFlight)
		res, err := (&http.Client{}).Get("http://" + app.Addr() + "/hold")
		if err == nil {
			_ = res.Body.Close()
		}
	}()
	<-reached

	closed := make(chan error, 1)
	go func() { closed <- stop() }()

	waitFor(t, "/readyz to report 503 after the gate closed", func() bool {
		code, _ := probe(app, "/readyz")
		return code == http.StatusServiceUnavailable
	})
	// The process still lives while it drains. A liveness probe must not
	// restart it.
	if code, _ := probe(app, "/healthz"); code != http.StatusOK {
		t.Fatalf("/healthz returned %d during the drain, want 200", code)
	}

	close(held)
	<-inFlight
	if err := <-closed; err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
}

func TestHealthzIsServedWithNoApplicationHandler(t *testing.T) {
	app := newApp(t, baseConfig(time.Second))
	stop := run(t, app)
	waitReady(t, app)
	if code, body := get(t, app, "/healthz"); code != http.StatusOK || !strings.Contains(body, "ok") {
		t.Fatalf("/healthz gave %d %q", code, body)
	}
	if err := stop(); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
}

func TestTheApplicationHandlerDoesNotShadowHealthz(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("the application answered"))
	})
	app := newApp(t, baseConfig(time.Second), host.WithHandler(mux))

	stop := run(t, app)
	waitReady(t, app)
	if _, body := get(t, app, "/healthz"); strings.Contains(body, "the application answered") {
		t.Fatalf("the application handler shadowed /healthz: %q", body)
	}
	if err := stop(); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
}
