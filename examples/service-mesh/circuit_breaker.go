package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/zoobzio/metricz"
)

// Circuit breaker metric keys
const (
	CircuitBreakerTripsKey metricz.Key = "circuit_breaker_trips"
)

// CircuitBreakerState represents circuit breaker states
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreakerConfig configures circuit breaker behavior
type CircuitBreakerConfig struct {
	Threshold   int           // Consecutive failures to open
	Timeout     time.Duration // Time before trying half-open
	HalfOpenMax int           // Max requests in half-open state
}

// CircuitBreaker implements the circuit breaker pattern with metrics
type CircuitBreaker struct {
	name   string
	config CircuitBreakerConfig

	mu              sync.RWMutex
	state           CircuitBreakerState
	failures        int
	successes       int
	lastFailureTime time.Time
	halfOpenCount   int

	// Metrics
	stateGauge     metricz.Gauge
	tripsCounter   metricz.Counter
	callsCounter   metricz.Counter
	successCounter metricz.Counter
	failureCounter metricz.Counter
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, registry *metricz.Registry, config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		name:           name,
		config:         config,
		state:          StateClosed,
		stateGauge:     registry.Gauge(metricz.Key(fmt.Sprintf("cb_%s", name))),
		tripsCounter:   registry.Counter(CircuitBreakerTripsKey),
		callsCounter:   registry.Counter(metricz.Key(fmt.Sprintf("cb_calls_%s", name))),
		successCounter: registry.Counter(metricz.Key(fmt.Sprintf("cb_success_%s", name))),
		failureCounter: registry.Counter(metricz.Key(fmt.Sprintf("cb_failure_%s", name))),
	}
}

// Allow checks if request should be allowed through
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.callsCounter.Inc()

	switch cb.state {
	case StateClosed:
		return true

	case StateOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailureTime) > cb.config.Timeout {
			cb.transitionToHalfOpen()
			return true
		}
		return false

	case StateHalfOpen:
		// Allow limited requests through
		if cb.halfOpenCount < cb.config.HalfOpenMax {
			cb.halfOpenCount++
			return true
		}
		return false
	}

	return false
}

// RecordSuccess records a successful call
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successCounter.Inc()
	cb.successes++

	switch cb.state {
	case StateHalfOpen:
		// Enough successes to close
		if cb.successes >= cb.config.HalfOpenMax {
			cb.transitionToClosed()
		}

	case StateClosed:
		// Reset failure count on success
		cb.failures = 0
	}
}

// RecordFailure records a failed call
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCounter.Inc()
	cb.failures++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		// Check if we should open
		if cb.failures >= cb.config.Threshold {
			cb.transitionToOpen()
		}

	case StateHalfOpen:
		// Failure in half-open immediately opens
		cb.transitionToOpen()
	}
}

// transitionToOpen moves to open state
func (cb *CircuitBreaker) transitionToOpen() {
	cb.state = StateOpen
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenCount = 0
	cb.stateGauge.Set(1.0) // 1 = open
	cb.tripsCounter.Inc()
}

// transitionToHalfOpen moves to half-open state
func (cb *CircuitBreaker) transitionToHalfOpen() {
	cb.state = StateHalfOpen
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenCount = 1   // Count this request
	cb.stateGauge.Set(0.5) // 0.5 = half-open
}

// transitionToClosed moves to closed state
func (cb *CircuitBreaker) transitionToClosed() {
	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenCount = 0
	cb.stateGauge.Set(0.0) // 0 = closed
}

// GetState returns current state
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset forces the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.transitionToClosed()
}
