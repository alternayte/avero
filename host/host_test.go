package host_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/avero/host"
)

func TestComponentsStartInOrderAndStopInReverseOrder(t *testing.T) {
	rec := &recorder{}
	a, b, c := newFake(rec, "a"), newFake(rec, "b"), newFake(rec, "c")
	app := newApp(t, baseConfig(2*time.Second), host.WithComponents(a, b, c))

	stop := run(t, app)
	waitReady(t, app)
	if err := stop(); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
	rec.equal(t, "start a", "start b", "start c", "stop c", "stop b", "stop a")
}

func TestAFailingStartStopsTheStartedComponentsInReverseOrder(t *testing.T) {
	rec := &recorder{}
	a, c := newFake(rec, "a"), newFake(rec, "c")
	b := newFake(rec, "b")
	b.startErr = errFake
	app := newApp(t, baseConfig(2*time.Second), host.WithComponents(a, b, c))

	err := runOnce(t, app)
	if err == nil {
		t.Fatal("Run returned no error for a failing Start")
	}
	// c never starts. b failed, so b does not stop. a stops.
	rec.equal(t, "start a", "start b", "stop a")
}

func TestAFailingStartNamesTheComponentAndStatesTheRepair(t *testing.T) {
	rec := &recorder{}
	b := newFake(rec, "broker")
	b.startErr = errFake
	app := newApp(t, baseConfig(2*time.Second), host.WithComponents(newFake(rec, "db"), b))

	err := runOnce(t, app)
	if err == nil {
		t.Fatal("Run returned no error for a failing Start")
	}
	if !strings.Contains(err.Error(), "broker") {
		t.Fatalf("the message does not name the component: %v", err)
	}
	if !errors.Is(err, errFake) {
		t.Fatalf("the error does not wrap the cause: %v", err)
	}
	if !strings.Contains(err.Error(), "→") {
		t.Fatalf("the error states no repair: %v", err)
	}
}

func TestExitReturnsOneForAStartFault(t *testing.T) {
	rec := &recorder{}
	b := newFake(rec, "broker")
	b.startErr = errFake
	app := newApp(t, baseConfig(2*time.Second), host.WithComponents(b))

	var out strings.Builder
	code := host.Exit(&out, runOnce(t, app))
	if code != 1 {
		t.Fatalf("Exit returned %d, want 1", code)
	}
	if !strings.Contains(out.String(), "broker") {
		t.Fatalf("the output does not name the component:\n%s", out.String())
	}
}

func TestExitReturnsZeroForNoError(t *testing.T) {
	var out strings.Builder
	if code := host.Exit(&out, nil); code != 0 {
		t.Fatalf("Exit returned %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Fatalf("Exit wrote %q, want nothing", out.String())
	}
}

func TestReadyzReturns503BeforeTheGateOpens(t *testing.T) {
	rec := &recorder{}
	slow := newFake(rec, "slow")
	slow.startBlocks = make(chan struct{})
	app := newApp(t, baseConfig(2*time.Second), host.WithComponents(slow))

	stop := run(t, app)
	waitServing(t, app)

	code, _ := get(t, app, "/readyz")
	if code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz returned %d before the gate opened, want 503", code)
	}
	close(slow.startBlocks)
	waitReady(t, app)
	if err := stop(); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
}

func TestReadyzReturns200WhenEveryReadyComponentReturnsNil(t *testing.T) {
	rec := &recorder{}
	app := newApp(t, baseConfig(2*time.Second),
		host.WithComponents(newFake(rec, "plain"), newReadyFake(rec, "ready")))

	stop := run(t, app)
	waitReady(t, app)
	code, _ := get(t, app, "/readyz")
	if code != http.StatusOK {
		t.Fatalf("/readyz returned %d, want 200", code)
	}
	if err := stop(); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
}

func TestReadyzReturns503WhenAReadyComponentReturnsAnError(t *testing.T) {
	rec := &recorder{}
	r := newReadyFake(rec, "broker")
	app := newApp(t, baseConfig(2*time.Second), host.WithComponents(r))

	stop := run(t, app)
	waitReady(t, app)

	r.setReady(errFake)
	code, body := get(t, app, "/readyz")
	if code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz returned %d, want 503", code)
	}
	if !strings.Contains(body, "broker") {
		t.Fatalf("the body does not name the component that is not ready: %q", body)
	}

	r.setReady(nil)
	if code, _ := get(t, app, "/readyz"); code != http.StatusOK {
		t.Fatalf("/readyz returned %d after the component recovered, want 200", code)
	}
	if err := stop(); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
}

func TestReadyzReturns503AfterTheGateCloses(t *testing.T) {
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

	// Hold one request open, so that the server keeps serving while the gate
	// closes. The drain waits for it.
	inFlight := make(chan struct{})
	go func() {
		defer close(inFlight)
		res, err := (&http.Client{}).Get("http://" + app.Addr() + "/hold")
		if err == nil {
			_, _ = io.Copy(io.Discard, res.Body)
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

	close(held)
	<-inFlight
	if err := <-closed; err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
	rec.equal(t, "start a", "stop a")
}

func TestAnInFlightRequestCompletesDuringShutdown(t *testing.T) {
	rec := &recorder{}
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte("done"))
	})
	app := newApp(t, baseConfig(5*time.Second),
		host.WithComponents(newFake(rec, "a")), host.WithHandler(mux))

	stop := run(t, app)
	waitReady(t, app)

	type result struct {
		code int
		body string
	}
	got := make(chan result, 1)
	go func() {
		code, body := get(t, app, "/slow")
		got <- result{code, body}
	}()

	// Give the request time to reach the handler, then close the application.
	time.Sleep(50 * time.Millisecond)
	err := stop()

	res := <-got
	if res.code != http.StatusOK || res.body != "done" {
		t.Fatalf("the in-flight request gave %d %q, want 200 \"done\"", res.code, res.body)
	}
	if err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
	rec.equal(t, "start a", "stop a")
}

func TestAHangingRequestDoesNotPreventExitWithinTheShutdownGrace(t *testing.T) {
	rec := &recorder{}
	grace := 300 * time.Millisecond
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	reached := make(chan struct{})
	var once bool
	mux := http.NewServeMux()
	mux.HandleFunc("/hang", func(w http.ResponseWriter, _ *http.Request) {
		if !once {
			once = true
			close(reached)
		}
		<-release
		_, _ = w.Write([]byte("late"))
	})
	log, read := captureLogger()
	app := host.New(baseConfig(grace),
		host.WithoutSignals(), host.WithLogger(log), host.WithListener(listener(t)),
		host.WithComponents(newFake(rec, "a")), host.WithHandler(mux))

	stop := run(t, app)
	waitReady(t, app)
	go func() {
		res, err := (&http.Client{}).Get("http://" + app.Addr() + "/hang")
		if err == nil {
			_ = res.Body.Close()
		}
	}()
	<-reached

	start := time.Now()
	err := stop()
	elapsed := time.Since(start)

	if elapsed > grace+2*time.Second {
		t.Fatalf("Run took %v, want a return within the grace of %v", elapsed, grace)
	}
	// The grace expiry is a step of the run sequence, not a fault. The process
	// exits 0. See the SDD, section 5.3, steps 9 and 11.
	if err != nil {
		t.Fatalf("Run returned an error for a cut request, want none: %v", err)
	}
	// The cut is never silent.
	out := read()
	if !strings.Contains(out, "level=WARN") {
		t.Fatalf("the cut request produced no warning:\n%s", out)
	}
	if !strings.Contains(out, "cut=1") {
		t.Fatalf("the warning does not count the cut requests:\n%s", out)
	}
	// The components still stop. A cut request must not skip the shutdown.
	rec.equal(t, "start a", "stop a")
}

func TestStopReceivesTheShutdownGraceDeadline(t *testing.T) {
	rec := &recorder{}
	grace := 4 * time.Second
	a := newFake(rec, "a")
	app := newApp(t, baseConfig(grace), host.WithComponents(a))

	stop := run(t, app)
	waitReady(t, app)
	before := time.Now()
	if err := stop(); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}

	deadline, ok := a.deadline()
	if !ok {
		t.Fatal("Stop received a context with no deadline")
	}
	if deadline.Before(before) || deadline.After(before.Add(grace+time.Second)) {
		t.Fatalf("the Stop deadline is %v, want a moment inside %v of %v", deadline, grace, before)
	}
}

func TestAPanicInStartIsNotRecovered(t *testing.T) {
	rec := &recorder{}
	bad := newFake(rec, "bad")
	bad.panicOnStart = true
	app := newApp(t, baseConfig(time.Second), host.WithComponents(newFake(rec, "a"), bad))

	panicked := make(chan any, 1)
	go func() {
		defer func() { panicked <- recover() }()
		_ = app.Run(context.Background())
	}()

	select {
	case v := <-panicked:
		if v == nil {
			t.Fatal("Run recovered the panic of a Start")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return within 5 seconds")
	}
}

func TestRunReturnsWhenTheContextIsCancelled(t *testing.T) {
	rec := &recorder{}
	app := newApp(t, baseConfig(time.Second), host.WithComponents(newFake(rec, "a")))
	stop := run(t, app)
	waitReady(t, app)
	if err := stop(); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
}

func TestAStopFaultIsReported(t *testing.T) {
	rec := &recorder{}
	a := newFake(rec, "a")
	a.stopErr = errFake
	app := newApp(t, baseConfig(2*time.Second), host.WithComponents(a))

	stop := run(t, app)
	waitReady(t, app)
	err := stop()
	if err == nil {
		t.Fatal("Run reported no fault although Stop failed")
	}
	if !strings.Contains(err.Error(), "a") || !errors.Is(err, errFake) {
		t.Fatalf("the error does not name the component or its cause: %v", err)
	}
}

func TestEveryComponentStopsAlthoughAnEarlierStopFailed(t *testing.T) {
	rec := &recorder{}
	a, b, c := newFake(rec, "a"), newFake(rec, "b"), newFake(rec, "c")
	b.stopErr = errFake
	app := newApp(t, baseConfig(2*time.Second), host.WithComponents(a, b, c))

	stop := run(t, app)
	waitReady(t, app)
	if err := stop(); err == nil {
		t.Fatal("Run reported no fault although Stop failed")
	}
	rec.equal(t, "start a", "start b", "start c", "stop c", "stop b", "stop a")
}

func TestTheApplicationHandlerServesItsRoutes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	app := newApp(t, baseConfig(time.Second), host.WithHandler(mux))

	stop := run(t, app)
	waitReady(t, app)
	code, body := get(t, app, "/hello")
	if code != http.StatusOK || body != "hello" {
		t.Fatalf("GET /hello gave %d %q", code, body)
	}
	if err := stop(); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
}

func TestReadyzIsServedWithNoApplicationHandler(t *testing.T) {
	app := newApp(t, baseConfig(time.Second))
	stop := run(t, app)
	waitReady(t, app)
	if err := stop(); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
}

func TestTheApplicationHandlerDoesNotShadowReadyz(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("the application answered"))
	})
	app := newApp(t, baseConfig(time.Second), host.WithHandler(mux))

	stop := run(t, app)
	waitReady(t, app)
	_, body := get(t, app, "/readyz")
	if strings.Contains(body, "the application answered") {
		t.Fatalf("the application handler shadowed /readyz: %q", body)
	}
	if err := stop(); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
}

func TestAddrReportsTheBoundAddress(t *testing.T) {
	app := newApp(t, baseConfig(time.Second))
	if !strings.HasPrefix(app.Addr(), "127.0.0.1:") {
		t.Fatalf("Addr returned %q", app.Addr())
	}
}

func TestRunRefusesASecondCall(t *testing.T) {
	rec := &recorder{}
	app := newApp(t, baseConfig(time.Second), host.WithComponents(newFake(rec, "a")))
	stop := run(t, app)
	waitReady(t, app)
	if err := stop(); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
	if err := runOnce(t, app); err == nil {
		t.Fatal("Run accepted a second call")
	}
}
