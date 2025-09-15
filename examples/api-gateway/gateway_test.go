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

	// Create handler
	handler := gateway.ProxyHandler("test-service", backend.URL)

	// Make request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify metrics were recorded
	counters := registry.GetCounters()
	if counter, exists := counters["requests_test-service"]; !exists {
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

			// Create handler
			handler := gateway.ProxyHandler("test", backend.URL)

			// Make request
			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			// Check status
			if rec.Code != sc.wantStatus {
				t.Errorf("Want status %d, got %d", sc.wantStatus, rec.Code)
			}

			// Check error counter
			counters := registry.GetCounters()
			errorCounter, hasErrors := counters["errors_test"]

			// Also check gateway's internal map
			if sc.wantError && !hasErrors {
				// Debug: list all counters
				t.Logf("Available counters in registry:")
				for name := range counters {
					t.Logf("  - %s: %f", name, counters[name].Value())
				}
				t.Logf("Gateway error map size: %d", len(gateway.errors))
				for name := range gateway.errors {
					t.Logf("  - gateway.errors[%s]", name)
				}
				t.Error("Expected error counter 'errors_test', not found")
			} else if sc.wantError && errorCounter.Value() != 1 {
				t.Errorf("Expected 1 error, got %f", errorCounter.Value())
			} else if !sc.wantError && hasErrors {
				t.Errorf("Unexpected error counter: %f", errorCounter.Value())
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

	handler := gateway.ProxyHandler("latency-test", backend.URL)

	// Make multiple requests
	for i := 0; i < len(latencies); i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// Check timer metrics
	timers := registry.GetTimers()
	timer, exists := timers["latency_latency-test"]
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

	handler := gateway.ProxyHandler("concurrent-test", backend.URL)

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

	// Verify all requests completed
	counters := registry.GetCounters()
	if counter, exists := counters["requests_concurrent-test"]; !exists {
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

	successHandler := gateway.ProxyHandler("healthy-service", successBackend.URL)

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

	errorHandler := gateway.ProxyHandler("failing-service", errorBackend.URL)

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

	// Check service-specific health
	if len(health.Services) != 2 {
		t.Errorf("Expected 2 services in health report, got %d",
			len(health.Services))
	}

	if failingHealth, exists := health.Services["failing-service"]; exists {
		if failingHealth.ErrorRate != 100.0 {
			t.Errorf("Failing service should have 100%% error rate, got %.2f%%",
				failingHealth.ErrorRate)
		}
	} else {
		t.Error("Failing service not in health report")
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

	// Verify Prometheus format
	expectedLines := []string{
		"# TYPE test_counter counter",
		"test_counter 42",
		"# TYPE test_gauge gauge",
		"test_gauge 3.14",
		"# TYPE test_timer_milliseconds histogram",
		"test_timer_milliseconds_count 1",
	}

	for _, expected := range expectedLines {
		if !contains(body, expected) {
			t.Errorf("Missing expected line: %s", expected)
		}
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

	handler := gateway.ProxyHandler("bench-service", backend.URL)

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
