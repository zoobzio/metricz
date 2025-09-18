# metricz

[![CI Status](https://github.com/zoobzio/metricz/workflows/CI/badge.svg)](https://github.com/zoobzio/metricz/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/zoobzio/metricz/graph/badge.svg?branch=main)](https://codecov.io/gh/zoobzio/metricz)
[![Go Report Card](https://goreportcard.com/badge/github.com/zoobzio/metricz)](https://goreportcard.com/report/github.com/zoobzio/metricz)
[![CodeQL](https://github.com/zoobzio/metricz/workflows/CodeQL/badge.svg)](https://github.com/zoobzio/metricz/security/code-scanning)
[![Go Reference](https://pkg.go.dev/badge/github.com/zoobzio/metricz.svg)](https://pkg.go.dev/github.com/zoobzio/metricz)
[![License](https://img.shields.io/github/license/zoobzio/metricz)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/zoobzio/metricz)](go.mod)
[![Release](https://img.shields.io/github/v/release/zoobzio/metricz)](https://github.com/zoobzio/metricz/releases)

High-performance metrics collection library for Go with compile-time safety and zero dependencies.

## Quick Start

```go
package main

import (
    "fmt"
    "time"
    "github.com/zoobzio/metricz"
)

// Define metrics as constants
const (
    RequestCount = metricz.Key("http_requests_total")
    ResponseTime = metricz.Key("http_response_duration_ms")
    ActiveConns  = metricz.Key("active_connections")
)

func main() {
    // Create isolated registry
    metrics := metricz.New()
    
    // Use typed metrics
    counter := metrics.Counter(RequestCount)
    timer := metrics.Timer(ResponseTime)
    gauge := metrics.Gauge(ActiveConns)
    
    // Thread-safe operations
    counter.Inc()
    counter.Add(5)
    
    gauge.Set(10)
    gauge.Inc()
    
    stopwatch := timer.Start()
    time.Sleep(100 * time.Millisecond)
    stopwatch.Stop()
    
    fmt.Printf("Requests: %.0f\n", counter.Value())
    fmt.Printf("Connections: %.0f\n", gauge.Value())
    fmt.Printf("Response time samples: %d\n", timer.Count())
}
```

## Core Features

### Performance
- **Sub-microsecond operations**: Atomic operations for metric updates
- **Zero allocations**: No memory allocations in steady-state operation
- **Lock-free updates**: Uses atomic operations for value changes
- **Minimal overhead**: Direct atomic access without intermediate layers

### Thread Safety
- All metric operations are thread-safe
- Multiple goroutines can update same metrics concurrently
- Registry operations use efficient read-write mutexes
- No external synchronization required

### Registry Isolation
- Each registry maintains completely isolated metrics
- No global state or singleton patterns
- Multiple registries can coexist without interference
- Perfect for multi-tenant or component isolation

### Compile-Time Safety
- All metrics require explicit Key type declaration
- Typos in metric names become compile errors
- No raw strings accepted by the API
- Consistent naming enforced at compile time

## API Reference

### Registry

Create and manage metric collections:

```go
// Create new registry
registry := metricz.New()

// Get or create metrics
counter := registry.Counter(key)
gauge := registry.Gauge(key)
histogram := registry.Histogram(key, buckets)
timer := registry.Timer(key)

// Reset all metrics (useful for testing)
registry.Reset()

// Export metrics
counters := registry.GetCounters()
gauges := registry.GetGauges()
histograms := registry.GetHistograms()
timers := registry.GetTimers()
```

### Counter

Monotonically increasing values:

```go
const RequestsTotal = metricz.Key("requests_total")

counter := registry.Counter(RequestsTotal)
counter.Inc()        // Increment by 1
counter.Add(5.5)     // Add positive value
value := counter.Value() // Read current value
```

### Gauge

Values that can increase or decrease:

```go
const QueueSize = metricz.Key("queue_size")

gauge := registry.Gauge(QueueSize)
gauge.Set(100)      // Set to specific value
gauge.Inc()         // Increment by 1
gauge.Dec()         // Decrement by 1
gauge.Add(10)       // Add value (can be negative)
value := gauge.Value() // Read current value
```

### Histogram

Track value distributions:

```go
const ResponseSize = metricz.Key("response_size_bytes")

// Define bucket boundaries
buckets := []float64{100, 500, 1000, 5000, 10000}
hist := registry.Histogram(ResponseSize, buckets)

// Record observations
hist.Observe(756.5)
hist.Observe(1234.0)

// Read statistics
buckets, counts := hist.Buckets() // Get distribution
sum := hist.Sum()                  // Total of all observations
count := hist.Count()              // Number of observations
overflow := hist.Overflow()        // Count exceeding largest bucket
```

### Timer

Measure durations with built-in histogram:

```go
const ProcessingTime = metricz.Key("processing_time_ms")

timer := registry.Timer(ProcessingTime)

// Method 1: Stopwatch pattern
stopwatch := timer.Start()
// ... do work ...
stopwatch.Stop()

// Method 2: Direct recording
timer.Record(150 * time.Millisecond)

// Custom buckets (in milliseconds)
buckets := []float64{1, 5, 10, 50, 100, 500, 1000}
timer := registry.TimerWithBuckets(ProcessingTime, buckets)

// Read statistics
count := timer.Count()             // Number of recordings
sum := timer.Sum()                 // Total duration in milliseconds
buckets, counts := timer.Buckets() // Distribution
```

## Export for Monitoring Systems

All registries provide export methods for Prometheus, StatsD, or custom monitoring:

```go
func exportMetrics(registry *metricz.Registry) {
    // Get all metrics as maps with string keys
    counters := registry.GetCounters()
    gauges := registry.GetGauges() 
    histograms := registry.GetHistograms()
    timers := registry.GetTimers()
    
    // Export to your monitoring system
    for name, counter := range counters {
        prometheus.CounterVec.WithLabelValues(name).Set(counter.Value())
    }
}
```

## Error Handling

metricz validates all inputs and ignores invalid values rather than panicking:

```go
counter.Add(-1.0)         // Ignored - counters can't decrease
counter.Add(math.NaN())   // Ignored - invalid value
counter.Add(math.Inf(1))  // Ignored - invalid value

gauge.Set(math.NaN())     // Ignored - invalid value

histogram.Observe(math.NaN())  // Ignored - invalid value
histogram.Observe(math.Inf(1)) // Ignored - invalid value
```

## Registry Patterns

### Service-Level Registry
```go
type APIService struct {
    registry *metricz.Registry
    requests metricz.Counter
    latency  metricz.Timer
}

func NewAPIService() *APIService {
    registry := metricz.New()
    return &APIService{
        registry: registry,
        requests: registry.Counter(HTTPRequestsTotal),
        latency:  registry.Timer(HTTPRequestLatency),
    }
}
```

### Multi-Registry Isolation
```go
func multiServiceExample() {
    // Complete isolation between services
    apiRegistry := metricz.New()
    dbRegistry := metricz.New()
    cacheRegistry := metricz.New()
    
    // Same key names, different registries - no conflicts
    apiCounter := apiRegistry.Counter(RequestsTotal)
    dbCounter := dbRegistry.Counter(RequestsTotal)
    cacheCounter := cacheRegistry.Counter(RequestsTotal)
    
    // Each tracks independently
    apiCounter.Add(100)   // API: 100
    dbCounter.Add(50)     // DB: 50  
    cacheCounter.Add(25)  // Cache: 25
}
```

## Installation

```bash
go get github.com/zoobzio/metricz
```

## Design Decisions

### Key Type Enforcement

The library enforces use of the `Key` type rather than raw strings to prevent metric naming errors that commonly occur in production systems. This design decision catches typos and naming inconsistencies at compile time rather than runtime.

```go
// Constants force consistent naming
const RequestCount = metricz.Key("requests_total")

// Compiler enforces correct usage
registry.Counter(RequestCount)     // ✓ Compiles
registry.Counter("requests_total") // ✗ Won't compile
```

### Registry Isolation

Each registry maintains completely isolated state to prevent metric collisions in complex applications. This allows different components or teams to maintain their own metrics without coordination.

### Atomic Operations

The library uses atomic operations (sync/atomic) for all metric value updates to achieve lock-free performance in hot paths. This provides sub-microsecond update times with zero lock contention.

### Zero Dependencies

Metricz depends only on the Go standard library (sync and time packages) to maximize compatibility and minimize security surface area. The optional clockz dependency enables deterministic testing.

### Bucket Design

Histograms and timers use pre-defined buckets rather than computing quantiles to maintain predictable memory usage and consistent performance regardless of observation count.

### No Global Registry

Global state creates dependency injection problems, testing complications, and prevents service isolation. Explicit registries make dependencies clear and testing straightforward. Each registry maintains complete isolation:

```go
// Clear dependency management
type Service struct {
    metrics *metricz.Registry
}

// Easy testing with fresh registries
func TestServiceMetrics(t *testing.T) {
    registry := metricz.New()
    // Test in isolation
}
```

### No Metric Labels/Tags

Labels multiply cardinality and can cause memory issues in production. For high-cardinality data, use multiple registries or separate keys. This design keeps the library simple, predictable, and prevents unbounded memory growth:

```go
// Instead of labels
// metric{service="api", method="GET", endpoint="/users"}

// Use explicit keys or separate registries
const (
    APIGetUsers  = metricz.Key("api_get_users")
    APIPostUsers = metricz.Key("api_post_users")
)
```

## Compatibility

- **Go version**: 1.23.2+
- **Dependencies**: None - pure Go standard library
- **Platforms**: All platforms supported by Go
- **API stability**: Semantic versioning with backwards compatibility guarantee