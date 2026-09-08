package host_test

import (
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/alternayte/avero/host"
)

func TestWithoutSignalsInstallsNoHandler(t *testing.T) {
	rec := &recorder{}
	app := newApp(t, baseConfig(2*time.Second), host.WithComponents(newFake(rec, "a")))
	if len(app.Signals()) != 0 {
		t.Fatalf("the application listens for %v, want no signal", app.Signals())
	}
	stop := run(t, app)
	waitReady(t, app)
	if err := stop(); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
}

func TestTheDefaultSignalsAreInterruptAndTerminate(t *testing.T) {
	app := host.New(baseConfig(time.Second))
	got := app.Signals()
	want := []os.Signal{os.Interrupt, syscall.SIGTERM}
	if len(got) != len(want) {
		t.Fatalf("the default signals are %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("the default signals are %v, want %v", got, want)
		}
	}
}

func TestNewKeepsTheConfiguration(t *testing.T) {
	cfg := baseConfig(7 * time.Second)
	cfg.Env = "production"
	app := host.New(cfg, host.WithListener(listener(t)))
	if app.Config().ShutdownGrace != 7*time.Second {
		t.Fatalf("the grace is %v, want 7s", app.Config().ShutdownGrace)
	}
	if app.Config().Env != "production" {
		t.Fatalf("the environment is %q, want production", app.Config().Env)
	}
}
