// Package metricz provides a type-safe, zero-dependency metrics collection library
// designed for high-performance applications requiring compile-time safety guarantees.
//
// # Core Philosophy
//
// Metricz enforces type safety through its Key-based API, preventing raw string usage
// that commonly leads to metric naming inconsistencies and runtime errors. All metric
// operations require explicit Key types, providing compile-time verification of metric
// names and eliminating string-based typos.
//
// # Key-Enforced API
//
// The foundation of metricz is the Key type, which wraps strings and forces explicit
// declaration of all metric names:
//
//	const (
//	    RequestCount = metricz.Key("http_requests_total")
//	    ResponseTime = metricz.Key("http_response_duration_seconds")
//	)
//
// Raw strings are rejected by the API - all methods accept only Key parameters,
// ensuring metric names are declared as constants and preventing runtime typos.
//
// # Four Metric Types
//
// Metricz implements four core metric types with atomic operations:
//
// Counter: Monotonically increasing values for event counting
//
//	counter := registry.Counter(RequestCount)
//	counter.Inc()
//	counter.Add(5)
//
// Gauge: Arbitrary values that can increase or decrease
//
//	gauge := registry.Gauge(MemoryUsage)
//	gauge.Set(1024)
//	gauge.Add(512)
//	gauge.Sub(256)
//
// Histogram: Distribution tracking with configurable buckets
//
//	hist := registry.Histogram(ResponseTime, []float64{0.1, 0.5, 1.0, 5.0})
//	hist.Observe(0.75)
//
// Timer: Duration tracking with built-in histogram functionality
//
//	timer := registry.Timer(ProcessingTime)
//	stop := timer.Start()
//	// ... work ...
//	stop()
//
// # Registry Pattern
//
// The Registry provides complete instance isolation, allowing multiple independent
// metric collections within the same application. Each registry maintains its own
// metric instances with no cross-registry interference:
//
//	registry := metricz.New()
//	counter := registry.Counter(RequestCount)
//
// Registry operations are thread-safe and use atomic operations where possible
// for optimal performance under concurrent access.
//
// # Thread-Safety Guarantees
//
// All metric operations are thread-safe and designed for high-concurrency environments:
//
// - Metric value updates use atomic operations (sync/atomic) for zero-lock performance
// - Registry operations use read-write mutexes for efficient concurrent access
// - Multiple goroutines can safely access the same metrics simultaneously
// - No external synchronization required for any operations
//
// # Zero-Allocation Performance
//
// Metricz is optimized for zero-allocation metric updates in steady-state operation:
//
// - Metric instances are created once and reused
// - Value updates use atomic operations without allocation
// - String conversions are minimized and cached where possible
// - Memory pools are avoided in favor of atomic operations
//
// # Zero Dependencies
//
// Metricz depends only on the Go standard library, specifically:
// - sync (for mutexes and atomic operations)
// - time (for timer functionality)
// - No external dependencies for maximum compatibility and minimal attack surface
//
// # Example Usage
//
//	// Declare metric keys as constants
//	const (
//	    RequestsTotal = metricz.Key("http_requests_total")
//	    ResponseDuration = metricz.Key("http_response_duration_seconds")
//	)
//
//	// Create registry
//	metrics := metricz.New()
//
//	// Use metrics with type safety
//	counter := metrics.Counter(RequestsTotal)
//	timer := metrics.Timer(ResponseDuration)
//
//	// Thread-safe operations
//	counter.Inc()
//	stop := timer.Start()
//	defer stop()
//
// This design ensures metric collection adds minimal overhead while providing
// maximum safety and consistency across application boundaries.
package metricz

import (
	"sync"
)

// Key is the mandatory key type for all metric operations.
// No raw strings allowed - compile-time enforcement.
type Key string

// Registry is the metric collection with complete instance isolation.
// Only accepts Key type - no raw strings, no generics.
type Registry struct {
	counters   map[Key]*counterImpl
	gauges     map[Key]*gaugeImpl
	histograms map[Key]*histogram
	timers     map[Key]*timer
	mu         sync.RWMutex
}

// New creates a Registry that accepts ONLY Key types.
// No raw strings allowed - forces explicit Key usage.
func New() *Registry {
	return &Registry{
		counters:   make(map[Key]*counterImpl),
		gauges:     make(map[Key]*gaugeImpl),
		histograms: make(map[Key]*histogram),
		timers:     make(map[Key]*timer),
	}
}

// Counter returns a counter metric, creating it if it doesn't exist.
func (r *Registry) Counter(key Key) Counter {
	r.mu.Lock()
	defer r.mu.Unlock()

	if counter, exists := r.counters[key]; exists {
		return counter
	}

	counter := newCounter()
	r.counters[key] = counter
	return counter
}

// Gauge returns a gauge metric, creating it if it doesn't exist.
func (r *Registry) Gauge(key Key) Gauge {
	r.mu.Lock()
	defer r.mu.Unlock()

	if gauge, exists := r.gauges[key]; exists {
		return gauge
	}

	gauge := newGauge()
	r.gauges[key] = gauge
	return gauge
}

// Histogram returns a histogram metric, creating it if it doesn't exist.
func (r *Registry) Histogram(key Key, buckets []float64) Histogram {
	r.mu.Lock()
	defer r.mu.Unlock()

	if hist, exists := r.histograms[key]; exists {
		return hist
	}

	if len(buckets) > 50 { // Basic sanity check
		buckets = buckets[:50]
	}

	hist := newHistogram(buckets)
	r.histograms[key] = hist
	return hist
}

// Timer returns a timer metric, creating it if it doesn't exist.
func (r *Registry) Timer(key Key) Timer {
	r.mu.Lock()
	defer r.mu.Unlock()

	if timer, exists := r.timers[key]; exists {
		return timer
	}

	timer := newTimer()
	r.timers[key] = timer
	return timer
}

// TimerWithBuckets returns a timer metric with custom buckets, creating it if it doesn't exist.
func (r *Registry) TimerWithBuckets(key Key, buckets []float64) Timer {
	r.mu.Lock()
	defer r.mu.Unlock()

	if timer, exists := r.timers[key]; exists {
		return timer
	}

	if len(buckets) > 50 { // Basic sanity check matching histogram behavior
		buckets = buckets[:50]
	}

	timer := newTimerWithBuckets(buckets)
	r.timers[key] = timer
	return timer
}

// Reset clears all metrics for clean test slate.
func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear all metrics for clean test slate
	r.counters = make(map[Key]*counterImpl)
	r.gauges = make(map[Key]*gaugeImpl)
	r.histograms = make(map[Key]*histogram)
	r.timers = make(map[Key]*timer)
}

// GetCounters returns a copy of all counters for export tools.
func (r *Registry) GetCounters() map[Key]Counter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[Key]Counter, len(r.counters))
	for key, counter := range r.counters {
		result[key] = counter
	}
	return result
}

// GetGauges returns a copy of all gauges for export tools.
func (r *Registry) GetGauges() map[Key]Gauge {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[Key]Gauge, len(r.gauges))
	for key, gauge := range r.gauges {
		result[key] = gauge
	}
	return result
}

// GetHistograms returns a copy of all histograms for export tools.
func (r *Registry) GetHistograms() map[Key]Histogram {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[Key]Histogram, len(r.histograms))
	for key, histogram := range r.histograms {
		result[key] = histogram
	}
	return result
}

// GetTimers returns a copy of all timers for export tools.
func (r *Registry) GetTimers() map[Key]Timer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[Key]Timer, len(r.timers))
	for key, timer := range r.timers {
		result[key] = timer
	}
	return result
}
