# metricz Testing Framework

Test patterns and integration utilities for comprehensive metrics testing.

## Directory Structure

```
testing/
├── integration/      # Integration test scenarios
│   ├── keys.go      # Centralized metric key constants
│   └── *_test.go    # Capability demonstrations  
└── reliability/     # Stress and boundary testing
    └── *_test.go    # Concurrency, memory, overflow tests
```

## Integration Tests

Integration tests demonstrate complete capabilities rather than isolated units. Each test tells a story about real-world usage.

### Key Management Pattern

All integration tests use centralized key constants from `keys.go`:

```go
import "metricz"

// Service metrics - type-safe keys
const (
    RequestsKey   metricz.Key = "requests"
    ErrorsKey     metricz.Key = "errors"  
    LatencyKey    metricz.Key = "latency"
)
```

This prevents key drift and ensures consistency across tests.

### Test Categories

#### Aggregation Testing (`aggregation_test.go`)
Tests metric aggregation from multiple sources. Worker registries collect local metrics, aggregator combines them.

**Pattern:** Distributed collection → Central aggregation → Derived metrics

```go
// Each worker has local registry
workers := make([]*metricz.Registry, numWorkers)

// Aggregate across all workers
for _, worker := range workers {
    totalTasks += worker.Counter("tasks.processed").Value()
    totalErrors += worker.Counter("tasks.errors").Value()
}

// Calculate derived metrics
errorRate := (totalErrors / totalTasks) * 100
aggregator.Gauge("error.rate.percent").Set(errorRate)
```

#### Export Patterns (`export_patterns_test.go`)
Demonstrates metric serialization for external systems.

**Prometheus Format:**
```go
fmt.Fprintf(&buf, "# TYPE %s counter\n", name)
fmt.Fprintf(&buf, "%s %.0f\n", name, counter.Value())
```

**JSON Export:**
```go
export := MetricExport{
    Timestamp: time.Now().UTC().Format(time.RFC3339),
    Counters:  make(map[string]float64),
    Gauges:    make(map[string]float64),
}
```

#### Isolation Testing (`isolation_test.go`)
Ensures complete metric isolation between test cases.

**Registry Per Test:**
```go
t.Run(name, func(t *testing.T) {
    registry := metricz.New() // Isolated registry
    // Test operations...
})
```

#### Memory Patterns (`memory_patterns_test.go`)
Verifies no memory leaks between registry instances.

**Cleanup Verification:**
```go
runtime.GC()
runtime.ReadMemStats(&memBefore)
// Create/destroy registries
runtime.GC()
runtime.ReadMemStats(&memAfter)
// Verify memory released
```

#### Race Detection (`race_test.go`)
Tests concurrent metric operations for race conditions.

**Concurrent Access Pattern:**
```go
var wg sync.WaitGroup
for i := 0; i < numWriters; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        registry.Counter(key).Inc()
    }()
}
```

#### Service Lifecycle (`service_lifecycle_test.go`)
Simulates real service metric patterns through startup, operation, and shutdown.

**Lifecycle Stages:**
1. Startup metrics (initialization timers)
2. Operational metrics (request processing)
3. Shutdown metrics (cleanup verification)

#### Test Patterns (`test_patterns_test.go`)
Common testing patterns for metrics.

**Table-Driven Tests:**
```go
tests := []struct {
    name           string
    operations     int
    errorRate      float64
    expectedHealth string
}{
    // Test cases...
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        registry := metricz.New()
        // Test with isolated registry
    })
}
```

**Parallel Test Isolation:**
```go
t.Run(name, func(t *testing.T) {
    t.Parallel() // Run in parallel
    registry := metricz.New() // Own registry
})
```

## Reliability Tests

Stress tests that push metrics to their limits.

### Concurrency Stress (`concurrency_stress_test.go`)

**CAS Loop Testing:**
Tests atomic operations under extreme contention.
```go
const goroutines = 1000
const operationsPerGoroutine = 10000
// Verify no starvation under load
```

**Reset During Load:**
Tests registry reset while under concurrent read/write load.

### Memory Pressure (`memory_pressure_test.go`)

**High Cardinality Testing:**
Creates many unique metric keys to test memory limits.
```go
for i := 0; i < 100000; i++ {
    key := fmt.Sprintf("metric_%d", i)
    registry.Counter(metricz.Key(key)).Inc()
}
```

### Overflow Testing (`overflow_test.go`)

**Numeric Limits:**
Tests behavior at numeric boundaries.
```go
counter.Add(math.MaxFloat64)
// Verify saturation, not overflow
```

## Test Helper Package

The `testing` package provides reusable utilities for testing metrics-instrumented code.

### Import Statement

```go
import "github.com/zoobzio/metricz/testing"
```

### Available Helpers

#### 1. Registry Factory

**NewTestRegistry** creates a registry with automatic cleanup:

```go
func TestMyFunction(t *testing.T) {
    registry := testing.NewTestRegistry(t)
    // Registry automatically reset after test completes
    
    // Use registry for test operations
    registry.Counter("operations").Inc()
}
```

**NewTestRegistries** creates multiple isolated registries:

```go
func TestMultiWorker(t *testing.T) {
    workers := testing.NewTestRegistries(t, 3)
    // Each registry gets individual cleanup
    
    // Use each registry independently
    for i, registry := range workers {
        key := fmt.Sprintf("worker.%d.tasks", i)
        registry.Counter(key).Inc()
    }
}
```

**Benefits:**
- Eliminates test contamination through automatic cleanup
- Uses `t.Cleanup()` for proper test lifecycle management
- Prevents registry state from affecting other tests
- Each registry gets individual cleanup handling

#### 2. Concurrent Load Generator

**GenerateLoad** standardizes concurrent stress testing:

```go
func TestConcurrentMetrics(t *testing.T) {
    registry := testing.NewTestRegistry(t)
    
    config := testing.LoadConfig{
        Workers:    10,
        Operations: 1000,
        Operation: func(workerID, opID int) {
            key := fmt.Sprintf("worker.%d.ops", workerID)
            registry.Counter(key).Inc()
        },
    }
    
    testing.GenerateLoad(t, config)
    
    // Verify concurrent operations completed successfully
    for i := 0; i < 10; i++ {
        key := fmt.Sprintf("worker.%d.ops", i)
        if registry.Counter(key).Value() != 1000 {
            t.Errorf("Worker %d: expected 1000 operations", i)
        }
    }
}
```

**Optional Setup Function:**
```go
config := testing.LoadConfig{
    Workers:    5,
    Operations: 100,
    Setup: func(workerID int) {
        // Per-worker initialization
        workerName := fmt.Sprintf("worker-%d", workerID)
        registry.Gauge(workerName + ".status").Set(1)
    },
    Operation: func(workerID, opID int) {
        // Per-operation logic
        registry.Counter("total.ops").Inc()
    },
}
```

**Benefits:**
- Eliminates WaitGroup boilerplate in tests
- Standardizes concurrent testing patterns
- Captures worker IDs properly to prevent closure issues
- Provides per-worker setup capabilities
- Makes stress testing reproducible and configurable

### Usage Patterns

Common patterns for using the test helpers effectively:

#### Basic Test Structure
```go
func TestWithIsolatedRegistry(t *testing.T) {
    // Get clean registry with automatic cleanup
    registry := testing.NewTestRegistry(t)
    
    // Perform test operations
    registry.Counter("operations").Inc()
    
    // Verify results
    if registry.Counter("operations").Value() != 1 {
        t.Error("Expected 1 operation")
    }
    // Cleanup happens automatically
}
```

#### Multi-Registry Testing
```go
func TestDistributedMetrics(t *testing.T) {
    // Create multiple isolated registries
    registries := testing.NewTestRegistries(t, 3)
    
    // Each registry operates independently
    for i, registry := range registries {
        key := fmt.Sprintf("node.%d.requests", i)
        registry.Counter(key).Inc()
    }
    
    // Verify isolation
    for i, registry := range registries {
        key := fmt.Sprintf("node.%d.requests", i)
        if registry.Counter(key).Value() != 1 {
            t.Errorf("Node %d: expected 1 request", i)
        }
    }
}
```

#### Concurrent Load Testing
```go
func TestUnderLoad(t *testing.T) {
    registry := testing.NewTestRegistry(t)
    
    // Configure concurrent load
    config := testing.LoadConfig{
        Workers:    50,
        Operations: 200,
        Operation: func(workerID, opID int) {
            registry.Counter("total.operations").Inc()
            registry.Gauge("active.workers").Set(float64(workerID))
        },
    }
    
    // Execute load test
    testing.GenerateLoad(t, config)
    
    // Verify expected total operations
    expected := float64(50 * 200) // workers * operations
    if registry.Counter("total.operations").Value() != expected {
        t.Errorf("Expected %f operations, got %f", 
            expected, registry.Counter("total.operations").Value())
    }
}
```

## Test Execution

### Run All Tests
```bash
go test ./testing/...
```

### Run with Race Detection
```bash
go test -race ./testing/...
```

### Run Specific Category
```bash
# Integration tests only
go test ./testing/integration/

# Reliability tests only  
go test ./testing/reliability/
```

### Benchmark Tests
```bash
go test -bench=. ./testing/...
```

## Best Practices

1. **Always use typed keys** - Use `metricz.Key` constants, not raw strings
2. **Isolate test registries** - Each test gets its own `metricz.New()`
3. **Clean up in parallel tests** - Use `t.Cleanup()` for resource cleanup
4. **Verify with actual values** - Don't just check non-zero, verify exact expectations
5. **Test degraded conditions** - Include failure scenarios, not just happy paths
6. **Use descriptive test names** - Test names should explain the capability being demonstrated

## Common Pitfalls

### String Key Usage (WRONG)
```go
// WRONG - loses type safety
registry.Counter("requests").Inc()
```

### Shared Registry (WRONG)
```go
// WRONG - tests contaminate each other
var globalRegistry = metricz.New()

func TestA(t *testing.T) {
    globalRegistry.Counter(key).Inc() // Affects TestB
}
```

### Missing Isolation (WRONG)
```go
// WRONG - parallel tests interfere
func TestParallel(t *testing.T) {
    t.Parallel()
    registry.Counter(sharedKey).Inc() // Race condition
}
```

## Correct Patterns

### Type-Safe Keys (RIGHT)
```go
// RIGHT - compile-time safety
const RequestsKey metricz.Key = "requests"
registry.Counter(RequestsKey).Inc()
```

### Test Isolation (RIGHT)
```go
// RIGHT - each test isolated
func TestA(t *testing.T) {
    registry := metricz.New() // Fresh registry
    registry.Counter(key).Inc()
}
```

### Parallel Safety (RIGHT)
```go
// RIGHT - isolated registries
func TestParallel(t *testing.T) {
    t.Parallel()
    registry := metricz.New() // Own registry
    registry.Counter(key).Inc()
}
```

## Contributing

When adding new test patterns:

1. Place integration tests in `testing/integration/`
2. Place stress tests in `testing/reliability/`
3. Add new keys to `keys.go` rather than inline strings
4. Document the pattern being tested
5. Include both success and failure scenarios
6. Ensure tests are deterministic and repeatable