package client

import (
	"errors"
	"sync"
	"time"
)

// ErrBreakerOpen states that the circuit is open, so the client sent no
// request. The call fails at once and the service gets time to recover.
var ErrBreakerOpen = errors.New("client: the circuit breaker is open")

// State is one state of the circuit breaker.
type State string

// The states of the circuit breaker.
const (
	// StateClosed passes every call.
	StateClosed State = "closed"
	// StateOpen fails every call at once.
	StateOpen State = "open"
	// StateHalfOpen passes one probe.
	StateHalfOpen State = "half-open"
)

// The defaults of the circuit breaker.
const (
	// DefaultThreshold is the number of failures in a row that opens the
	// circuit.
	DefaultThreshold = 5
	// DefaultCooldown is the time that the circuit stays open.
	DefaultCooldown = 30 * time.Second
)

// BreakerConfig states one circuit breaker.
type BreakerConfig struct {
	// Threshold is the number of failures in a row that opens the circuit.
	// A value below one takes the default.
	Threshold int
	// Cooldown is the time that the circuit stays open. A value below one
	// takes the default.
	Cooldown time.Duration
	// Now returns the time. A test replaces it.
	Now func() time.Time
}

// Breaker is the circuit breaker of one client.
//
// The state machine holds three states. Closed passes every call and counts
// the failures in a row. The failure that reaches the threshold opens the
// circuit. Open fails every call with ErrBreakerOpen until the cooldown ends,
// and then it passes one probe as half-open. A probe that succeeds closes the
// circuit. A probe that fails opens it again for another cooldown.
type Breaker struct {
	threshold int
	cooldown  time.Duration
	now       func() time.Time

	mu       sync.Mutex
	state    State
	failures int
	openedAt time.Time
	probing  bool
}

// NewBreaker builds a circuit breaker.
func NewBreaker(cfg BreakerConfig) *Breaker {
	b := &Breaker{
		threshold: cfg.Threshold,
		cooldown:  cfg.Cooldown,
		now:       cfg.Now,
		state:     StateClosed,
	}
	if b.threshold < 1 {
		b.threshold = DefaultThreshold
	}
	if b.cooldown < 1 {
		b.cooldown = DefaultCooldown
	}
	if b.now == nil {
		b.now = time.Now
	}
	return b
}

// State returns the state of the circuit. It moves an open circuit to
// half-open when the cooldown ended.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.settle()
	return b.state
}

// Allow reports whether a call can go out. It returns ErrBreakerOpen when the
// circuit is open, and it takes the probe of a half-open circuit.
func (b *Breaker) Allow() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.settle()
	switch b.state {
	case StateOpen:
		return ErrBreakerOpen
	case StateHalfOpen:
		if b.probing {
			return ErrBreakerOpen
		}
		b.probing = true
		return nil
	default:
		return nil
	}
}

// Success records a call that reached an answer.
func (b *Breaker) Success() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.probing = false
	b.state = StateClosed
}

// Failure records a call that failed. The failure that reaches the threshold
// opens the circuit, and a failed probe opens it again.
func (b *Breaker) Failure() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == StateHalfOpen {
		b.open()
		return
	}
	b.failures++
	if b.failures >= b.threshold {
		b.open()
	}
}

// open puts the circuit in the open state.
func (b *Breaker) open() {
	b.state = StateOpen
	b.openedAt = b.now()
	b.probing = false
}

// settle moves an open circuit to half-open when the cooldown ended. The
// caller holds the lock.
func (b *Breaker) settle() {
	if b.state == StateOpen && b.now().Sub(b.openedAt) >= b.cooldown {
		b.state = StateHalfOpen
		b.probing = false
	}
}
