package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zoobzio/metricz"
)

// TestBlackFridayScenario tests the circuit breaker pattern under load.
func TestBlackFridayScenario(t *testing.T) {
	// Story: Payment service degradation during Black Friday
	// This test verifies that circuit breakers prevent cascade failures

	registry := metricz.New()

	// Metrics for tracking the incident
	paymentLatency := registry.Timer("payment_latency")
	paymentErrors := registry.Counter("payment_errors")
	circuitBreakerTrips := registry.Counter("circuit_breaker_trips")
	requestsSaved := registry.Counter("requests_saved_by_circuit_breaker")

	// Simulate payment service
	var degraded atomic.Bool
	var requestCount atomic.Int64

	paymentService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)

		if degraded.Load() {
			// Service is degraded - high latency and errors
			time.Sleep(500 * time.Millisecond) // Extreme latency
			http.Error(w, "Payment provider timeout", http.StatusGatewayTimeout)
			return
		}

		// Normal operation
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer paymentService.Close()

	// Circuit breaker implementation
	type CircuitBreaker struct {
		failureCount int
		isOpen       bool
		mu           sync.Mutex
	}

	cb := &CircuitBreaker{}

	// Gateway with circuit breaker
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Check circuit breaker
		cb.mu.Lock()
		if cb.isOpen {
			cb.mu.Unlock()
			requestsSaved.Inc()
			http.Error(w, "Circuit breaker open", http.StatusServiceUnavailable)
			return
		}
		cb.mu.Unlock()

		// Proxy to payment service
		stopwatch := paymentLatency.Start()
		req, err := http.NewRequestWithContext(context.Background(), "GET", paymentService.URL, http.NoBody)
		if err != nil {
			http.Error(w, "Failed to create request", http.StatusInternalServerError)
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		stopwatch.Stop()

		if err != nil || resp.StatusCode != http.StatusOK {
			paymentErrors.Inc()

			// Trip circuit breaker after 5 failures
			cb.mu.Lock()
			cb.failureCount++
			if cb.failureCount >= 5 && !cb.isOpen {
				cb.isOpen = true
				circuitBreakerTrips.Inc()
			}
			cb.mu.Unlock()

			if err != nil {
				http.Error(w, "Payment service error", http.StatusBadGateway)
			} else {
				w.WriteHeader(resp.StatusCode)
			}
			return
		}

		// Reset failure count on success
		cb.mu.Lock()
		cb.failureCount = 0
		cb.mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer gateway.Close()

	// Phase 1: Normal traffic
	client := &http.Client{Timeout: 2 * time.Second}

	for i := 0; i < 10; i++ {
		req, err := http.NewRequestWithContext(context.Background(), "GET", gateway.URL, http.NoBody)
		if err != nil {
			t.Logf("Failed to create request: %v", err)
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Request failed: %v", err)
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	normalRequests := requestCount.Load()
	if normalRequests != 10 {
		t.Errorf("Expected 10 requests to payment service, got %d", normalRequests)
	}

	// Phase 2: Payment service degrades
	degraded.Store(true)
	requestCount.Store(0)

	// Simulate retry storm
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 3; j++ { // Each client retries 3 times
				req, err := http.NewRequestWithContext(context.Background(), "GET", gateway.URL, http.NoBody)
				if err != nil {
					t.Logf("Failed to create request: %v", err)
					continue
				}
				resp, err := client.Do(req)
				if err != nil {
					t.Logf("Request failed: %v", err)
				}
				if resp != nil {
					resp.Body.Close()
				}
			}
		}()
	}

	wg.Wait()

	// Verify circuit breaker engaged
	if circuitBreakerTrips.Value() == 0 {
		t.Error("Circuit breaker should have tripped")
	}

	// Verify requests were saved from hitting degraded service
	if requestsSaved.Value() == 0 {
		t.Error("Circuit breaker should have saved some requests")
	}

	// Verify not all 60 requests hit the payment service
	stormRequests := requestCount.Load()
	if stormRequests >= 60 {
		t.Errorf("Circuit breaker didn't prevent retry storm: %d requests hit service", stormRequests)
	}

	// Phase 3: Recovery
	degraded.Store(false)
	cb.mu.Lock()
	cb.isOpen = false
	cb.failureCount = 0
	cb.mu.Unlock()

	requestCount.Store(0)

	// Traffic resumes normally
	for i := 0; i < 10; i++ {
		req, err := http.NewRequestWithContext(context.Background(), "GET", gateway.URL, http.NoBody)
		if err != nil {
			t.Logf("Failed to create request: %v", err)
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Request failed: %v", err)
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	recoveryRequests := requestCount.Load()
	if recoveryRequests != 10 {
		t.Errorf("Expected 10 requests after recovery, got %d", recoveryRequests)
	}

	// Analyze metrics
	avgLatency := paymentLatency.Sum() / float64(paymentLatency.Count())
	errorRate := paymentErrors.Value() / float64(paymentLatency.Count()) * 100

	t.Logf("Black Friday Incident Analysis:")
	t.Logf("  Average latency: %.1fms", avgLatency)
	t.Logf("  Error rate: %.1f%%", errorRate)
	t.Logf("  Circuit breaker trips: %.0f", circuitBreakerTrips.Value())
	t.Logf("  Requests saved: %.0f", requestsSaved.Value())
	t.Logf("  Prevented %.0f unnecessary retries", 60-float64(stormRequests))
}
