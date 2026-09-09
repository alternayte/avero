package client_test

import (
	"errors"
	"testing"
	"time"

	"github.com/alternayte/avero/codegen/client"
)

// clock returns a time that a test moves by hand.
type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

func TestTheBreakerRunsItsStateMachine(t *testing.T) {
	c := &clock{now: time.Unix(0, 0)}
	b := client.NewBreaker(client.BreakerConfig{Threshold: 2, Cooldown: time.Minute, Now: c.Now})

	// Closed. A failure below the threshold keeps it closed.
	if b.State() != client.StateClosed {
		t.Fatalf("the breaker starts %s, want closed", b.State())
	}
	b.Failure()
	if b.State() != client.StateClosed {
		t.Fatalf("one failure gave %s, want closed", b.State())
	}
	// A success clears the count, so the next single failure keeps it
	// closed.
	b.Success()
	b.Failure()
	if b.State() != client.StateClosed {
		t.Fatalf("the count did not clear: %s", b.State())
	}

	// The failure that reaches the threshold opens the circuit.
	b.Failure()
	if b.State() != client.StateOpen {
		t.Fatalf("two failures gave %s, want open", b.State())
	}
	if err := b.Allow(); !errors.Is(err, client.ErrBreakerOpen) {
		t.Fatalf("Allow returned %v, want ErrBreakerOpen", err)
	}

	// The cooldown ends. The circuit passes one probe and no more.
	c.now = c.now.Add(time.Minute)
	if b.State() != client.StateHalfOpen {
		t.Fatalf("after the cooldown the breaker is %s, want half-open", b.State())
	}
	if err := b.Allow(); err != nil {
		t.Fatalf("the probe returned %v, want nil", err)
	}
	if err := b.Allow(); !errors.Is(err, client.ErrBreakerOpen) {
		t.Fatalf("a second probe returned %v, want ErrBreakerOpen", err)
	}

	// A failed probe opens the circuit again.
	b.Failure()
	if b.State() != client.StateOpen {
		t.Fatalf("the failed probe gave %s, want open", b.State())
	}

	// A probe that succeeds closes the circuit.
	c.now = c.now.Add(time.Minute)
	if err := b.Allow(); err != nil {
		t.Fatalf("the probe returned %v, want nil", err)
	}
	b.Success()
	if b.State() != client.StateClosed {
		t.Fatalf("the breaker is %s, want closed", b.State())
	}
	if err := b.Allow(); err != nil {
		t.Fatalf("Allow returned %v, want nil", err)
	}
}

func TestANilBreakerPassesEveryCall(t *testing.T) {
	var b *client.Breaker
	if err := b.Allow(); err != nil {
		t.Fatalf("Allow returned %v, want nil", err)
	}
	b.Failure()
	b.Success()
}
