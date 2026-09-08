//go:build unix

package host_test

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/alternayte/avero/host"
)

func TestASignalClosesTheApplication(t *testing.T) {
	rec := &recorder{}
	// SIGUSR1 is safe in a test. SIGTERM and SIGINT would end the test binary
	// if the handler were absent.
	app := host.New(baseConfig(2*time.Second),
		host.WithLogger(quietLogger()),
		host.WithListener(listener(t)),
		host.WithSignals(syscall.SIGUSR1),
		host.WithComponents(newFake(rec, "a")))

	errs := make(chan error, 1)
	go func() { errs <- app.Run(context.Background()) }()
	waitReady(t, app)

	if err := syscall.Kill(os.Getpid(), syscall.SIGUSR1); err != nil {
		t.Fatalf("Kill returned an error: %v", err)
	}
	select {
	case err := <-errs:
		if err != nil {
			t.Fatalf("Run returned an error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the signal did not close the application")
	}
	rec.equal(t, "start a", "stop a")
}
