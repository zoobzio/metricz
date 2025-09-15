# metricz

[![CI Status](https://github.com/zoobzio/metricz/workflows/CI/badge.svg)](https://github.com/zoobzio/metricz/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/zoobzio/metricz/graph/badge.svg?branch=main)](https://codecov.io/gh/zoobzio/metricz)
[![Go Report Card](https://goreportcard.com/badge/github.com/zoobzio/metricz)](https://goreportcard.com/report/github.com/zoobzio/metricz)
[![CodeQL](https://github.com/zoobzio/metricz/workflows/CodeQL/badge.svg)](https://github.com/zoobzio/metricz/security/code-scanning)
[![Go Reference](https://pkg.go.dev/badge/github.com/zoobzio/metricz.svg)](https://pkg.go.dev/github.com/zoobzio/metricz)
[![License](https://img.shields.io/github/license/zoobzio/metricz)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/zoobzio/metricz)](go.mod)
[![Release](https://img.shields.io/github/v/release/zoobzio/metricz)](https://github.com/zoobzio/metricz/releases)

Zero-dependency metrics collection for Go applications that prevents runtime key errors through compile-time type safety.

## Quick Start

```go
package main

import (
    "time"
    "github.com/zoobzio/metricz"
)

// Define metric keys as compile-time constants
const (
    RequestsTotal metricz.Key = "requests_total"
    RequestLatency metricz.Key = "request_latency"
    ActiveUsers metricz.Key = "active_users"
)

func main() {
    // Create isolated registry instance
    registry := metricz.New()
    
    // Use typed keys - no raw strings allowed
    counter := registry.Counter(RequestsTotal)
    timer := registry.Timer(RequestLatency)
    gauge := registry.Gauge(ActiveUsers)
    
    // Record metrics
    counter.Inc()
    stopwatch := timer.Start()
    time.Sleep(10 * time.Millisecond)
    stopwatch.Stop()
    gauge.Set(42)
    
    // Values are immediately available
    println("Requests:", counter.Value())
    println("Users:", gauge.Value())
}
```

## Why This Exists

Most Go metrics libraries accept raw strings for metric names, creating runtime errors from typos and making refactoring dangerous. When you rename a metric, every string reference must be manually found and updated - miss one and you lose data silently.

metricz solves this through the `Key` type:
- **Compile-time safety**: Typos become compile errors, not silent data loss
- **Refactoring confidence**: Rename a key constant and all usages update automatically  
- **Zero runtime overhead**: Keys are just strings internally
- **Complete isolation**: Each registry is independent with no global state

## Key Features

- **Type-safe metric keys**: No raw strings - all keys must be `metricz.Key` constants
- **Instance isolation**: Multiple registries with independent metric namespaces
- **Thread-safe operations**: All metrics support concurrent access without external locking
- **Zero dependencies**: Pure Go implementation with no external requirements
- **Comprehensive metric types**: Counters, Gauges, Histograms, and Timers with sensible defaults

## Installation

```bash
go get github.com/zoobzio/metricz
```

## Metric Types

### Counter
Monotonically increasing values that only go up:

```go
const RequestsProcessed metricz.Key = "requests_processed"

counter := registry.Counter(RequestsProcessed)
counter.Inc()           // Add 1
counter.Add(5.0)        // Add 5
value := counter.Value() // Get current total
```

### Gauge  
Values that can increase and decrease:

```go
const QueueDepth metricz.Key = "queue_depth"

gauge := registry.Gauge(QueueDepth)
gauge.Set(10.0)    // Set absolute value
gauge.Inc()        // Increment by 1
gauge.Dec()        // Decrement by 1
gauge.Add(-3.0)    // Add/subtract any value
value := gauge.Value() // Get current value
```

### Histogram
Distribution tracking with configurable buckets:

```go
const ResponseSize metricz.Key = "response_size_bytes"

// Custom buckets for response sizes
buckets := []float64{100, 500, 1000, 5000, 10000}
histogram := registry.Histogram(ResponseSize, buckets)

histogram.Observe(1500.0) // Record observation
sum := histogram.Sum()    // Total of all observations
count := histogram.Count() // Number of observations
```

### Timer
Duration tracking built on histograms:

```go
const DatabaseLatency metricz.Key = "database_latency"

timer := registry.Timer(DatabaseLatency)

// Manual timing
timer.Record(150 * time.Millisecond)

// Automatic timing with stopwatch
stopwatch := timer.Start()
// ... do work ...
stopwatch.Stop() // Automatically records duration

// Access histogram data
sum := timer.Sum()    // Total milliseconds
count := timer.Count() // Number of operations
```

## The Key Type Requirement

**Critical**: All metric operations require `metricz.Key` type - raw strings are rejected at compile time.

### Right Way - Define Constants

```go
// Define all keys as constants for compile-time safety
const (
    UserRegistrations metricz.Key = "user_registrations"
    LoginAttempts     metricz.Key = "login_attempts" 
    SessionDuration   metricz.Key = "session_duration"
)

func trackUserActivity(registry *metricz.Registry) {
    registry.Counter(UserRegistrations).Inc()
    registry.Timer(SessionDuration).Record(5 * time.Minute)
}
```

### Wrong Way - Raw Strings (Won't Compile)

```go
// This will cause compile errors
func badExample(registry *metricz.Registry) {
    registry.Counter("user_registrations").Inc() // ❌ Compile error
    registry.Timer("session_duration").Record(5 * time.Minute) // ❌ Compile error
}
```

### Why This Matters

```go
// With string-based libraries - silent bugs
requests := registry.Counter("http_requests_total")
requests.Inc()

// Later, typo creates new metric instead of updating existing
reqs := registry.Counter("http_request_total") // Missing 's' - creates new metric!
reqs.Inc() // Data goes to wrong metric

// With metricz - compile errors prevent bugs  
const HTTPRequestsTotal metricz.Key = "http_requests_total"
requests := registry.Counter(HTTPRequestsTotal)
reqs := registry.Counter(HTTPRequestsTotal) // Same constant - same metric guaranteed
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
timer.Record(-time.Second) // Ignored - negative duration
```

## Thread Safety

All operations are thread-safe without external locking:

```go
const WorkerMetric metricz.Key = "worker_operations"

func workerExample() {
    registry := metricz.New()
    counter := registry.Counter(WorkerMetric)
    
    // Safe to call from multiple goroutines
    for i := 0; i < 100; i++ {
        go func() {
            counter.Inc() // Thread-safe increment
        }()
    }
}
```

## Examples

See the [examples/](examples/) directory for complete applications:

- **[batch-processor/](examples/batch-processor/)** - Comprehensive batch processing with stage metrics, failure tracking, and performance analysis
- **[api-gateway/](examples/api-gateway/)** - HTTP API with request tracking, latency monitoring, and error analysis  
- **[service-mesh/](examples/service-mesh/)** - Microservice communication with circuit breakers and load balancing metrics
- **[worker-pool/](examples/worker-pool/)** - Concurrent job processing with worker utilization and queue depth tracking

## Design Decisions

### Why Key Type Instead of Strings?

String-based metrics lead to typos, silent failures, and refactoring dangers. The `Key` type provides compile-time verification with zero runtime cost.

### Why Registry Struct Not Interface?

Concrete structs are faster, simpler to use, and avoid interface boxing overhead. The registry API is stable and doesn't require multiple implementations.

### Why No Global Registry?

Global state creates dependency injection problems, testing complications, and prevents service isolation. Explicit registries make dependencies clear and testing straightforward.

### Why No Metric Labels/Tags?

Labels multiply cardinality and can cause memory issues. For high-cardinality data, use multiple registries or separate keys. This keeps the library simple and predictable.

## Performance

Measured on AMD Ryzen 5 3600X, Go 1.23.2:

| Operation | Time | Memory |
|-----------|------|--------|
| Counter.Inc() | 4.6ns | 0 allocs |
| Gauge.Set() | 5.0ns | 0 allocs |
| Histogram.Observe() | 30.6ns | 0 allocs |

Real-world HTTP tracking: 200ns with 32B allocation (request metadata).

### Key Characteristics
- **Zero allocation updates**: All metric operations allocate 0 bytes after creation
- **Lock-free reads**: Value() operations never block or wait
- **Atomic operations**: Minimal contention even with 100+ concurrent writers
- **Constant time lookup**: Registry operations in O(1) time

### Verify Performance
```bash
# Core operations
go test -bench="Counter_Inc$|Gauge_Set$|Histogram_Observe$" -benchmem ./testing/benchmarks/

# Real usage pattern  
go test -bench=BenchmarkHTTPRequestTracking -benchmem ./testing/benchmarks/
```

See [testing/benchmarks/](testing/benchmarks/) for comprehensive performance analysis.

## Documentation

- [API Documentation](https://pkg.go.dev/github.com/zoobzio/metricz) - Complete API reference
- [Examples](examples/) - Working applications showing real-world usage
- [Architecture](docs/architecture.md) - Design decisions and trade-offs  
- [Testing](testing/) - Integration and reliability tests

## Compatibility

- **Go version**: 1.23.2+
- **Dependencies**: None - pure Go standard library
- **Platforms**: All platforms supported by Go
- **API stability**: Semantic versioning with backwards compatibility guarantee