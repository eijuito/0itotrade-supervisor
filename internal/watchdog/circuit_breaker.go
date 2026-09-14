package watchdog

import (
	"math/rand"
	"sync"
	"time"
)

type CircuitState string

const (
	StateClosed   CircuitState = "CLOSED"    // Normal operation
	StateOpen     CircuitState = "OPEN"      // Tripped due to repeated failures
	StateHalfOpen CircuitState = "HALF_OPEN" // Testing recovery
)

// CircuitBreaker enforces exponential backoff with jitter and trip limits.
type CircuitBreaker struct {
	mu             sync.Mutex
	state          CircuitState
	failures       int
	maxFailures    int
	baseBackoff    time.Duration
	maxBackoff     time.Duration
	currentBackoff time.Duration
	lastFailure    time.Time
}

func NewCircuitBreaker(maxFailures int, baseBackoff, maxBackoff time.Duration) *CircuitBreaker {
	if maxFailures <= 0 {
		maxFailures = 5
	}
	if baseBackoff <= 0 {
		baseBackoff = 2 * time.Second
	}
	if maxBackoff <= 0 {
		maxBackoff = 60 * time.Second
	}

	return &CircuitBreaker{
		state:          StateClosed,
		maxFailures:    maxFailures,
		baseBackoff:    baseBackoff,
		maxBackoff:     maxBackoff,
		currentBackoff: baseBackoff,
	}
}

// CanExecute checks if an operation is allowed by the breaker state.
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateClosed {
		return true
	}

	// In OPEN state, check if backoff period has passed to transition to HALF_OPEN
	if cb.state == StateOpen {
		if time.Since(cb.lastFailure) >= cb.currentBackoff {
			cb.state = StateHalfOpen
			return true
		}
		return false
	}

	return true // HALF_OPEN allows probe test
}

// RecordSuccess resets failures and closes the breaker.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.state = StateClosed
	cb.currentBackoff = cb.baseBackoff
}

// RecordFailure increments failures, calculates backoff with jitter, and trips if threshold met.
func (cb *CircuitBreaker) RecordFailure() (trippedNow bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	// Calculate exponential backoff: base * 2^(failures-1) with jitter
	backoff := cb.baseBackoff * time.Duration(1<<(min(cb.failures-1, 6)))
	if backoff > cb.maxBackoff {
		backoff = cb.maxBackoff
	}

	// Add random jitter between 0% and 25%
	jitter := time.Duration(rand.Int63n(int64(backoff / 4)))
	cb.currentBackoff = backoff + jitter

	if cb.failures >= cb.maxFailures && cb.state != StateOpen {
		cb.state = StateOpen
		return true
	}

	return false
}

func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

func (cb *CircuitBreaker) Failures() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failures
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
