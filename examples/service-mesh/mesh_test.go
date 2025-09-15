package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/zoobzio/metricz"
)

func TestServiceMeshBasic(t *testing.T) {
	registry := metricz.New()
	mesh := NewServiceMesh(registry)

	// Register a mock service
	service := NewMockService("test-service", 10*time.Millisecond, 0.1)
	mesh.RegisterService("test-service", "test-1", service)

	mesh.Start()
	defer mesh.Stop()

	// Make a request
	req := ServiceRequest{
		From:    "test-client",
		To:      "test-service",
		Method:  "GET",
		Path:    "/test",
		TraceID: "test-trace-1",
	}

	resp := mesh.Route(req)

	if !resp.Success {
		if resp.Error != nil {
			// May have hit the 10% error rate
			t.Logf("Request failed (expected occasionally): %v", resp.Error)
		}
	}

	// Verify metrics
	counters := registry.GetCounters()
	if requests, exists := counters[metricz.Key("requests_test-service")]; !exists {
		t.Error("Request counter not found")
	} else if requests.Value() != 1 {
		t.Errorf("Expected 1 request, got %f", requests.Value())
	}
}

func TestServiceMeshLoadBalancing(t *testing.T) {
	registry := metricz.New()
	mesh := NewServiceMesh(registry)

	// Register multiple instances
	for i := 0; i < 3; i++ {
		service := NewMockService("lb-service", 5*time.Millisecond, 0)
		mesh.RegisterService("lb-service", fmt.Sprintf("lb-%d", i), service)
	}

	mesh.Start()
	defer mesh.Stop()

	// Make multiple requests
	numRequests := 30
	for i := 0; i < numRequests; i++ {
		req := ServiceRequest{
			From:    "test-client",
			To:      "lb-service",
			Method:  "GET",
			Path:    "/test",
			TraceID: fmt.Sprintf("lb-trace-%d", i),
		}
		mesh.Route(req)
	}

	// Check distribution
	counters := registry.GetCounters()

	var totalInstanceRequests float64
	instanceCounts := make(map[string]float64)

	for i := 0; i < 3; i++ {
		key := metricz.Key(fmt.Sprintf("instance_lb-service_lb-%d", i))
		if counter, exists := counters[key]; exists {
			count := counter.Value()
			instanceCounts[fmt.Sprintf("lb-%d", i)] = count
			totalInstanceRequests += count
		}
	}

	// With round-robin, each should get approximately equal requests
	expectedPerInstance := float64(numRequests) / 3
	tolerance := expectedPerInstance * 0.3 // 30% tolerance

	for instance, count := range instanceCounts {
		if count < expectedPerInstance-tolerance || count > expectedPerInstance+tolerance {
			t.Errorf("Instance %s: expected ~%.0f requests, got %.0f",
				instance, expectedPerInstance, count)
		}
	}
}

func TestCircuitBreaker(t *testing.T) {
	registry := metricz.New()
	mesh := NewServiceMesh(registry)

	// Register a service that will fail
	service := NewMockService("failing-service", 5*time.Millisecond, 1.0) // 100% error rate
	mesh.RegisterService("failing-service", "fail-1", service)

	mesh.Start()
	defer mesh.Stop()

	// Make requests until circuit breaker opens
	for i := 0; i < 10; i++ {
		req := ServiceRequest{
			From:    "test-client",
			To:      "failing-service",
			Method:  "GET",
			Path:    "/test",
			TraceID: fmt.Sprintf("cb-trace-%d", i),
		}
		resp := mesh.Route(req)

		if i < 5 {
			// First 5 should fail normally
			if resp.Success {
				t.Errorf("Request %d: expected failure, got success", i)
			}
		} else {
			// After threshold, circuit breaker should open
			if resp.Error != nil && resp.Error.Error() == "circuit breaker open for failing-service" {
				// Expected - circuit breaker opened
				break
			}
		}
	}

	// Verify circuit breaker metrics
	counters := registry.GetCounters()
	if trips, exists := counters[metricz.Key("circuit_breaker_trips")]; !exists {
		t.Error("Circuit breaker trips counter not found")
	} else if trips.Value() == 0 {
		t.Error("Circuit breaker should have tripped")
	}
}

func TestServiceMeshRetry(t *testing.T) {
	registry := metricz.New()
	mesh := NewServiceMesh(registry)

	// Register a service with moderate error rate
	service := NewMockService("retry-service", 5*time.Millisecond, 0.5) // 50% error rate
	mesh.RegisterService("retry-service", "retry-1", service)

	mesh.Start()
	defer mesh.Stop()

	// Make several requests
	successCount := 0
	for i := 0; i < 10; i++ {
		req := ServiceRequest{
			From:    "test-client",
			To:      "retry-service",
			Method:  "GET",
			Path:    "/test",
			TraceID: fmt.Sprintf("retry-trace-%d", i),
		}
		resp := mesh.Route(req)
		if resp.Success {
			successCount++
		}
	}

	// With 50% error rate and retries, success rate should be higher than 50%
	successRate := float64(successCount) / 10.0
	if successRate <= 0.5 {
		t.Errorf("Success rate too low with retries: %.2f", successRate)
	}

	// Check retry metrics
	counters := registry.GetCounters()
	if retries, exists := counters[metricz.Key("retries_total")]; exists && retries.Value() > 0 {
		t.Logf("Total retries: %.0f", retries.Value())

		if retriesSuccess, exists := counters[metricz.Key("retries_success")]; exists {
			t.Logf("Successful retries: %.0f", retriesSuccess.Value())
		}
	}
}

func TestServiceMeshHealthCheck(t *testing.T) {
	registry := metricz.New()
	mesh := NewServiceMesh(registry)

	// Register healthy and unhealthy instances
	healthy := NewMockService("health-service", 5*time.Millisecond, 0)
	unhealthy := NewMockService("health-service", 5*time.Millisecond, 0)
	unhealthy.SetHealthy(false)

	mesh.RegisterService("health-service", "healthy-1", healthy)
	mesh.RegisterService("health-service", "unhealthy-1", unhealthy)

	mesh.Start()
	defer mesh.Stop()

	// Make requests - should only go to healthy instance
	for i := 0; i < 10; i++ {
		req := ServiceRequest{
			From:    "test-client",
			To:      "health-service",
			Method:  "GET",
			Path:    "/test",
			TraceID: fmt.Sprintf("health-trace-%d", i),
		}
		resp := mesh.Route(req)

		if !resp.Success {
			t.Errorf("Request failed despite healthy instance: %v", resp.Error)
		}

		// All requests should go to healthy instance
		if resp.Instance != "healthy-1" {
			t.Errorf("Request routed to wrong instance: %s", resp.Instance)
		}
	}

	// Verify only healthy instance received requests
	counters := registry.GetCounters()

	if unhealthyCounter, exists := counters[metricz.Key("instance_health-service_unhealthy-1")]; exists {
		if unhealthyCounter.Value() > 0 {
			t.Errorf("Unhealthy instance received %.0f requests", unhealthyCounter.Value())
		}
	}
}

// Benchmark mesh routing
func BenchmarkServiceMesh(b *testing.B) {
	registry := metricz.New()
	mesh := NewServiceMesh(registry)

	// Register multiple services
	for i := 0; i < 5; i++ {
		service := NewMockService("bench-service", 1*time.Millisecond, 0.01)
		mesh.RegisterService("bench-service", fmt.Sprintf("bench-%d", i), service)
	}

	mesh.Start()
	defer mesh.Stop()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			req := ServiceRequest{
				From:    "bench-client",
				To:      "bench-service",
				Method:  "GET",
				Path:    "/bench",
				TraceID: fmt.Sprintf("bench-%d", i),
			}
			mesh.Route(req)
			i++
		}
	})
}
