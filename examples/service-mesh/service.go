package main

import (
	"context"
	"errors"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zoobzio/metricz"
)

// MockService simulates a service with configurable behavior
type MockService struct {
	name      string
	latency   time.Duration
	errorRate float64

	mu      sync.RWMutex
	healthy bool
	stopped bool
}

// NewMockService creates a mock service instance
func NewMockService(name string, latency time.Duration, errorRate float64) *MockService {
	return &MockService{
		name:      name,
		latency:   latency,
		errorRate: errorRate,
		healthy:   true,
	}
}

// Call simulates a service call
func (s *MockService) Call(req ServiceRequest) ServiceResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.stopped {
		return ServiceResponse{
			Success: false,
			Error:   errors.New("service stopped"),
		}
	}

	// Simulate processing time
	processingTime := s.latency

	// Add some variance (±20%)
	variance := time.Duration(float64(processingTime) * (0.8 + rand.Float64()*0.4))
	time.Sleep(variance)

	// Simulate errors based on error rate
	if rand.Float64() < s.errorRate {
		errorType := rand.Intn(3)
		var err error

		switch errorType {
		case 0:
			err = errors.New("internal server error")
		case 1:
			err = errors.New("timeout")
		case 2:
			err = errors.New("service unavailable")
		}

		return ServiceResponse{
			Success: false,
			Error:   err,
		}
	}

	// Successful response
	return ServiceResponse{
		Success: true,
		Data: map[string]interface{}{
			"service":  s.name,
			"method":   req.Method,
			"path":     req.Path,
			"trace_id": req.TraceID,
		},
	}
}

// Health returns service health status
func (s *MockService) Health() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.healthy && !s.stopped
}

// SetHealthy sets service health
func (s *MockService) SetHealthy(healthy bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.healthy = healthy
}

// SetErrorRate updates error rate
func (s *MockService) SetErrorRate(rate float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errorRate = rate
}

// SetLatency updates service latency
func (s *MockService) SetLatency(latency time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latency = latency
}

// Stop stops the service
func (s *MockService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
}

// AuthService simulates authentication service with potential goroutine leaks
type AuthService struct {
	*MockService
	registry       *metricz.Registry
	goroutineGauge metricz.Gauge
	memoryGauge    metricz.Gauge
	leakMode       int32 // 0=normal, 1=leak mode
	leakedRoutines int64
}

// NewAuthService creates an auth service with metrics tracking
func NewAuthService(registry *metricz.Registry) *AuthService {
	return &AuthService{
		MockService:    NewMockService("auth-service", 25*time.Millisecond, 0.02),
		registry:       registry,
		goroutineGauge: registry.Gauge("auth_goroutines"),
		memoryGauge:    registry.Gauge("auth_memory_mb"),
	}
}

// Call overrides MockService.Call to add auth-specific behavior
func (a *AuthService) Call(req ServiceRequest) ServiceResponse {
	// Track goroutines and memory
	a.trackSystemMetrics()

	// Simulate token validation with potential leak
	if atomic.LoadInt32(&a.leakMode) == 1 {
		a.simulateTokenValidationLeak(req)
	}

	return a.MockService.Call(req)
}

// simulateTokenValidationLeak creates goroutines that don't clean up properly
func (a *AuthService) simulateTokenValidationLeak(req ServiceRequest) {
	// Create validation goroutine that gets stuck
	go func() {
		atomic.AddInt64(&a.leakedRoutines, 1)

		// Simulate JWT validation that hangs on network call
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// This select will block forever because we never send to done channel
		done := make(chan bool)
		select {
		case <-ctx.Done():
			// Timeout - normal cleanup
			return
		case <-done:
			// This never happens - goroutine leak
			return
		}
	}()

	// Also create a metrics collection goroutine that accumulates
	go func() {
		atomic.AddInt64(&a.leakedRoutines, 1)

		ticker := time.NewTicker(1 * time.Millisecond)
		// Deliberately forget to stop ticker - leak

		for range ticker.C {
			// Collect metrics forever
			_ = runtime.NumGoroutine()
		}
	}()
}

// trackSystemMetrics updates goroutine and memory metrics
func (a *AuthService) trackSystemMetrics() {
	goroutines := runtime.NumGoroutine()
	a.goroutineGauge.Set(float64(goroutines))

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memoryMB := float64(m.Alloc) / 1024 / 1024
	a.memoryGauge.Set(memoryMB)
}

// EnableLeakMode turns on goroutine leak simulation
func (a *AuthService) EnableLeakMode() {
	atomic.StoreInt32(&a.leakMode, 1)
}

// DisableLeakMode turns off goroutine leak simulation
func (a *AuthService) DisableLeakMode() {
	atomic.StoreInt32(&a.leakMode, 0)
}

// GetLeakedRoutines returns number of leaked goroutines created
func (a *AuthService) GetLeakedRoutines() int64 {
	return atomic.LoadInt64(&a.leakedRoutines)
}
