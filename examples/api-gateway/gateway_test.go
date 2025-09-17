package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/zoobzio/metricz"
)

func TestGatewayIsolation(t *testing.T) {
	// Each test gets its own registry - complete isolation
	registry := metricz.New()
	gateway := NewGateway(registry)

	// Create test backend
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	}))
	defer backend.Close()

	// Create handler - use predefined service "auth"
	handler := gateway.ProxyHandler("auth", backend.URL)

	// Make request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify metrics were recorded - use Key type
	counters := registry.GetCounters()
	if counter, exists := counters[AuthServiceRequestsKey]; !exists {
		t.Fatal("Request counter not created")
	} else if counter.Value() != 1 {
		t.Errorf("Expected 1 request, got %f", counter.Value())
	}

	// Reset registry for next test
	registry.Reset()

	// Verify clean slate
	counters = registry.GetCounters()
	if len(counters) != 0 {
		t.Errorf("Registry not clean after reset: %d counters remain", len(counters))
	}
}

func TestGatewayErrorTracking(t *testing.T) {

	scenarios := []struct {
		name       string
		backend    http.HandlerFunc
		wantStatus int
		wantError  bool
	}{
		{
			name: "success",
			backend: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name: "not_found",
			backend: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantError:  true,
		},
		{
			name: "internal_error",
			backend: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  true,
		},
		{
			name: "timeout",
			backend: func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(10 * time.Second) // Force timeout
			},
			wantStatus: http.StatusGatewayTimeout,
			wantError:  true,
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			// Create fresh registry and gateway for each test
			registry := metricz.New()
			gateway := NewGateway(registry)

			// Create backend
			backend := httptest.NewServer(sc.backend)
			defer backend.Close()

			// Create handler - use predefined service "auth"
			handler := gateway.ProxyHandler("auth", backend.URL)

			// Make request
			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			// Check status
			if rec.Code != sc.wantStatus {
				t.Errorf("Want status %d, got %d", sc.wantStatus, rec.Code)
			}

			// Check error counter - use Key type
			counters := registry.GetCounters()
			errorCounter, hasErrors := counters[AuthServiceErrorsKey]

			// Check error counter
			if sc.wantError && !hasErrors {
				// Debug: list all counters
				t.Logf("Available counters in registry:")
				for key := range counters {
					t.Logf("  - %s: %f", key, counters[key].Value())
				}
				t.Error("Expected error counter for auth service, not found")
			} else if sc.wantError && errorCounter.Value() != 1 {
				t.Errorf("Expected 1 error, got %f", errorCounter.Value())
			} else if !sc.wantError && hasErrors && errorCounter.Value() != 0 {
				// Counter exists (created at startup) but should be zero
				t.Errorf("Expected 0 errors, got %f", errorCounter.Value())
			}
		})
	}
}

func TestGatewayLatencyTracking(t *testing.T) {
	registry := metricz.New()
	gateway := NewGateway(registry)

	// Backend with controlled latency
	latencies := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
	}

	requestNum := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestNum < len(latencies) {
			time.Sleep(latencies[requestNum])
			requestNum++
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	// Use predefined service "user"
	handler := gateway.ProxyHandler("user", backend.URL)

	// Make multiple requests
	for i := 0; i < len(latencies); i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// Check timer metrics - use Key type
	timers := registry.GetTimers()
	timer, exists := timers[UserServiceLatencyKey]
	if !exists {
		t.Fatal("Latency timer not created")
	}

	if timer.Count() != uint64(len(latencies)) {
		t.Errorf("Expected %d recordings, got %d", len(latencies), timer.Count())
	}

	// Verify reasonable average (should be around 30ms)
	avg := timer.Sum() / float64(timer.Count())
	if avg < 25 || avg > 35 {
		t.Errorf("Average latency out of expected range: %.2fms", avg)
	}
}

func TestGatewayConcurrentRequests(t *testing.T) {
	registry := metricz.New()
	gateway := NewGateway(registry)

	// Backend that holds connections
	var activeConnections int32
	var maxConnections int32
	var mu sync.Mutex

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		activeConnections++
		if activeConnections > maxConnections {
			maxConnections = activeConnections
		}
		mu.Unlock()

		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		activeConnections--
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	// Use predefined service "order"
	handler := gateway.ProxyHandler("order", backend.URL)

	// Launch concurrent requests
	var wg sync.WaitGroup
	numRequests := 20

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}()
	}

	// Check active requests gauge during execution
	time.Sleep(25 * time.Millisecond) // Let requests start

	activeGauge := gateway.activeRequests.Value()
	if activeGauge <= 0 {
		t.Errorf("Expected positive active requests, got %f", activeGauge)
	}

	wg.Wait()

	// Verify all requests completed - use Key type
	counters := registry.GetCounters()
	if counter, exists := counters[OrderServiceRequestsKey]; !exists {
		t.Fatal("Request counter not found")
	} else if counter.Value() != float64(numRequests) {
		t.Errorf("Expected %d requests, got %f", numRequests, counter.Value())
	}

	// Active requests should be zero after completion
	if gateway.activeRequests.Value() != 0 {
		t.Errorf("Active requests not zero after completion: %f",
			gateway.activeRequests.Value())
	}
}

func TestGatewayHealthCheck(t *testing.T) {
	registry := metricz.New()
	gateway := NewGateway(registry)

	// Initial health - should be healthy with no traffic
	health := gateway.GetHealth()
	if !health.Healthy {
		t.Error("Gateway should be healthy initially")
	}

	// Simulate successful requests
	successBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer successBackend.Close()

	// Use predefined service "auth"
	successHandler := gateway.ProxyHandler("auth", successBackend.URL)

	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		successHandler.ServeHTTP(rec, req)
	}

	// Should still be healthy
	health = gateway.GetHealth()
	if !health.Healthy {
		t.Errorf("Gateway should be healthy with 0%% errors, got %.2f%%",
			health.ErrorRate)
	}

	// Simulate errors
	errorBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer errorBackend.Close()

	// Use predefined service "payment" for error testing
	errorHandler := gateway.ProxyHandler("payment", errorBackend.URL)

	// Generate 10% error rate (10 errors after 100 successes)
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		errorHandler.ServeHTTP(rec, req)
	}

	// Should be unhealthy with >5% errors
	health = gateway.GetHealth()
	if health.Healthy {
		t.Errorf("Gateway should be unhealthy with %.2f%% errors",
			health.ErrorRate)
	}

	// Check service-specific health - always 4 predefined services
	if len(health.Services) != 4 {
		t.Errorf("Expected 4 services in health report, got %d",
			len(health.Services))
	}

	if failingHealth, exists := health.Services["payment"]; exists {
		if failingHealth.ErrorRate != 100.0 {
			t.Errorf("Failing service should have 100%% error rate, got %.2f%%",
				failingHealth.ErrorRate)
		}
	} else {
		t.Error("Payment service not in health report")
	}
}

func TestMetricsExport(t *testing.T) {
	registry := metricz.New()

	// Test metric keys - the RIGHT way
	const (
		TestCounterKey metricz.Key = "test_counter"
		TestGaugeKey   metricz.Key = "test_gauge"
		TestTimerKey   metricz.Key = "test_timer"
	)

	// Create some metrics
	registry.Counter(TestCounterKey).Add(42)
	registry.Gauge(TestGaugeKey).Set(3.14)
	registry.Timer(TestTimerKey).Record(100 * time.Millisecond)

	// Create metrics handler
	handler := MetricsHandler(registry)

	// Make request
	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Check response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	// Verify JSON format (MetricsHandler returns JSON counts)
	expectedJSON := `{"counters":1,"gauges":1,"timers":1,"histograms":0}`
	if body != expectedJSON {
		t.Errorf("Expected JSON %s, got %s", expectedJSON, body)
	}
}

// Benchmark concurrent request handling
func BenchmarkGatewayConcurrent(b *testing.B) {
	registry := metricz.New()
	gateway := NewGateway(registry)

	// Fast backend
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "OK")
	}))
	defer backend.Close()

	// Use predefined service "auth" for benchmarking
	handler := gateway.ProxyHandler("auth", backend.URL)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && contains(s[1:], substr)
}
