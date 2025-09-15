# metricz Performance Benchmarks

**Author**: CRASH  
**Date**: 2025-09-12  
**Purpose**: Comprehensive performance measurement and validation for the metricz library

This directory contains realistic performance benchmarks that validate the library's performance claims and identify bottlenecks under real-world conditions.

## Philosophy

> "If it ain't measured under load, it ain't measured at all."

These benchmarks focus on real-world performance patterns, not toy examples. Every benchmark reflects actual usage scenarios found in production systems. No vanity metrics, no bullshit numbers - just honest performance data.

## Benchmark Organization

### Core Performance Tests

**[`core_bench_test.go`](core_bench_test.go)** - Individual metric operation performance
- Counter operations: `Inc()`, `Add()`, `Value()`
- Gauge operations: `Set()`, `Add()`, `Value()`  
- Histogram operations: `Observe()`, `Sum()`, `Count()`
- Timer operations: `Start()`, `Stop()`
- Sequential vs parallel performance comparison
- Atomic CAS contention testing

**Performance Standards:**
- Counter `Inc()`: < 50ns/op, 0 allocs/op
- Gauge `Set()`: < 50ns/op, 0 allocs/op
- Histogram `Observe()`: < 200ns/op, 0 allocs/op
- Timer record: < 250ns/op combined

### Registry and Concurrency Tests

**[`registry_bench_test.go`](registry_bench_test.go)** - Registry performance and lock contention
- Metric creation vs retrieval performance
- Export operations: `GetCounters()`, `GetGauges()`, etc.
- Registry mutex contention under mixed read/write loads
- Registry reset performance
- Mixed workload performance (90% updates, 10% exports)

**Key Scenarios:**
- Export operations shouldn't block metric updates after creation
- Registry lock contention analysis under high concurrency
- Performance degradation curves with increasing metric cardinality

### Real-World Application Patterns

**[`scenarios_bench_test.go`](scenarios_bench_test.go)** - Production usage patterns
- HTTP request tracking (from api-gateway example)
- Batch processing metrics (from batch-processor example)
- Worker pool utilization (from worker-pool example)
- Service mesh monitoring (from service-mesh example)
- Mixed load with 80/20 normal/spike traffic patterns
- Long-running service with periodic exports

**Realistic Load Patterns:**
- Normal request processing with error rates
- Batch job processing with queue management
- Worker pool utilization tracking
- Circuit breaker and health check patterns

### Memory and Resource Tests

**[`memory_bench_test.go`](memory_bench_test.go)** - Memory efficiency and allocation validation
- Zero-allocation verification for metric updates
- Performance under memory pressure
- GC impact during operations
- Memory leak detection across registry lifecycle
- Large registry memory usage patterns
- Allocation tracking for all metric types

**Memory Validation:**
- Counter/Gauge updates: 0 allocs/op after creation
- Memory growth should be linear with metric count
- No memory leaks during registry lifecycle
- Performance degradation under memory pressure

### Stress and Edge Cases

**[`stress_bench_test.go`](stress_bench_test.go)** - Extreme conditions and edge cases
- Extreme concurrency (10-500+ goroutines)
- High cardinality metrics (10,000+ unique keys)
- Large bucket histograms (50 buckets)
- Continuous export load testing
- Memory exhaustion behavior
- Goroutine leak detection
- Long-running stability testing

**Stress Scenarios:**
- Breaking point identification for atomic operations
- Performance degradation curves under extreme load
- System behavior at resource limits
- Stability over extended periods

## Comparison Benchmarks

The [`comparison/`](comparison/) directory contains isolated benchmarks against popular alternatives. This module has its own `go.mod` to prevent dependency pollution in the main library.

### Why Isolated?

> "Don't want prometheus pulling in 47 dependencies just to prove we're faster. Keep that shit quarantined."

The comparison module isolates third-party dependencies to:
- Prevent supply chain vulnerabilities in main module
- Avoid bloating the main binary
- Maintain clean dependency tree for the library

### Benchmark Targets

**vs Prometheus** [`prometheus_vs_test.go`](comparison/prometheus_vs_test.go)
- Counter, Gauge, and Histogram performance comparison
- Parallel operation performance
- Memory usage and allocation patterns
- Registry creation and high cardinality scenarios

**vs Expvar** [`expvar_vs_test.go`](comparison/expvar_vs_test.go)
- Standard library performance comparison
- Int and Float operations
- Creation and reading performance
- Memory efficiency comparison

**vs go-metrics** [`gometrics_vs_test.go`](comparison/gometrics_vs_test.go)
- rcrowley/go-metrics performance comparison
- Counter, Gauge, Histogram, and Timer operations
- Mixed operations and high cardinality tests
- Memory footprint comparison

## Running the Benchmarks

### Prerequisites

```bash
# Ensure you're in the metricz project root
cd /home/zoobzio/code/metricz

# Run benchmarks from the main module
go test -bench=. ./testing/benchmarks/

# Run with memory allocation reporting
go test -bench=. -benchmem ./testing/benchmarks/

# Run specific benchmark categories
go test -bench=BenchmarkCounter ./testing/benchmarks/
go test -bench=BenchmarkRegistry ./testing/benchmarks/
go test -bench=BenchmarkHTTP ./testing/benchmarks/
```

### Comparison Benchmarks

```bash
# Enter the comparison module
cd testing/benchmarks/comparison

# Download dependencies (isolated from main module)
go mod download

# Run comparison benchmarks
go test -bench=. -benchmem

# Run specific comparisons
go test -bench=BenchmarkCounter.*Metricz
go test -bench=BenchmarkCounter.*Prometheus
go test -bench=BenchmarkMixed.*
```

### Advanced Profiling

```bash
# CPU profiling
go test -bench=BenchmarkStress -cpuprofile=cpu.prof ./testing/benchmarks/
go tool pprof cpu.prof

# Memory profiling
go test -bench=BenchmarkMemory -memprofile=mem.prof ./testing/benchmarks/
go tool pprof mem.prof

# Trace analysis
go test -bench=BenchmarkConcurrency -trace=trace.out ./testing/benchmarks/
go tool trace trace.out

# Block profiling (lock contention)
go test -bench=BenchmarkRegistry -blockprofile=block.prof ./testing/benchmarks/
go tool pprof block.prof
```

## Quick Start - Understanding Your Performance

### Most Important Numbers

If you only run three benchmarks, run these:

```bash
# Core operations - what you'll use most
go test -bench="BenchmarkCounter_Inc$|BenchmarkGauge_Set$|BenchmarkHistogram_Observe$" -benchmem

# Real usage pattern - HTTP request tracking  
go test -bench=BenchmarkHTTPRequestTracking -benchmem

# Comparison - how we stack up
cd comparison && go test -bench="BenchmarkCounterOperations.*Metricz" -benchmem
```

Expected output (AMD Ryzen 5 3600X, Go 1.23.2):
```
BenchmarkCounter_Inc-12           260694387    4.594 ns/op    0 B/op    0 allocs/op
BenchmarkGauge_Set-12             239400040    5.005 ns/op    0 B/op    0 allocs/op  
BenchmarkHistogram_Observe-12      38880819   30.63 ns/op    0 B/op    0 allocs/op
BenchmarkHTTPRequestTracking-12     5444191  199.9 ns/op    32 B/op    1 allocs/op

BenchmarkCounterOperations_Metricz-12  258933044    4.603 ns/op    0 B/op    0 allocs/op
```

### What These Numbers Mean

- **4.594 ns/op**: Each counter increment takes ~4.6 nanoseconds (217M ops/second)
- **0 B/op, 0 allocs/op**: Zero memory allocation per operation
- **30.63 ns/op**: Histogram operations ~31ns (32M ops/second)
- **HTTP benchmark**: Complete request cycle (counter + timer + gauge) in ~200ns
- **32 B/op, 1 allocs/op**: HTTP tracking allocates for request context, not metrics

### Performance Interpretation Guide

**Good Performance Indicators**:
- Counter/Gauge operations < 50ns/op
- Zero allocations for all metric updates  
- Parallel performance shows scaling improvement
- Memory usage grows linearly with metric count

**Warning Signs**:
- Operations > 100ns/op (investigate contention)
- Any allocations during metric updates (memory leak risk)
- Parallel worse than sequential (lock contention)
- Memory growth faster than linear (cardinality explosion)

## Benchmark Results Interpretation

### Performance Metrics

**ns/op** - Nanoseconds per operation
- Lower is better
- Counter operations should be < 50ns/op
- Histogram operations should be < 200ns/op

**allocs/op** - Allocations per operation  
- Should be 0 for metric updates after creation
- Only metric creation should allocate

**B/op** - Bytes allocated per operation
- Should be 0 for metric updates
- Tracks memory efficiency

**MB/s** - Throughput for data processing benchmarks
- Higher is better for batch operations

### Concurrency Analysis

**Parallel vs Sequential Performance**
- Parallel should scale with available CPUs
- Atomic operations may show contention at high concurrency
- Registry operations may show lock contention

**Contention Detection**
- Performance degradation with increasing goroutines
- Block profiling reveals lock contention hotspots
- Atomic CAS loop spinning under extreme contention

### Memory Patterns

**Zero Allocation Validation**
- Metric updates should show 0 allocs/op
- Only metric creation allocates memory
- Memory growth should be linear with metric count

**Memory Pressure Impact**
- Performance degradation under low memory
- GC impact on operation latency
- Memory leak detection across lifecycle

## Performance Baselines

These are the performance characteristics validated by the benchmarks:

### Single-Threaded Performance

| Operation | Target | Typical |
|-----------|---------|---------|
| Counter.Inc() | < 50ns/op | ~25ns/op |
| Counter.Add() | < 75ns/op | ~35ns/op |
| Gauge.Set() | < 50ns/op | ~25ns/op |
| Histogram.Observe() | < 200ns/op | ~150ns/op |
| Timer Start/Stop | < 250ns/op | ~200ns/op |

### Parallel Performance

| Operation | Scaling | Notes |
|-----------|---------|-------|
| Counter operations | Linear to 8-16 cores | CAS contention at 50+ concurrent |
| Gauge operations | Linear to 8-16 cores | Similar to counters |
| Histogram operations | Limited by RWMutex | Per-histogram locking |
| Registry exports | Blocks metric creation | Read lock contention |

### Memory Characteristics

| Metric Type | Creation Cost | Update Cost | Memory/Metric |
|-------------|---------------|-------------|---------------|
| Counter | ~48B + key | 0 allocs | ~80B total |
| Gauge | ~48B + key | 0 allocs | ~80B total |
| Histogram | ~200B + buckets | 0 allocs | ~400B + buckets |
| Timer | ~250B + histogram | 0 allocs | ~500B total |

## Interpreting Benchmark Output

### Sample Output
```
BenchmarkCounter_Inc-8                    50000000    25.2 ns/op    0 B/op    0 allocs/op
BenchmarkCounter_Inc_Parallel-8          200000000     8.5 ns/op    0 B/op    0 allocs/op
BenchmarkHistogram_Observe-8             10000000    156.3 ns/op    0 B/op    0 allocs/op
BenchmarkRegistry_Export_Under_Load-8     5000000    312.1 ns/op   48 B/op    1 allocs/op
```

**Analysis:**
- Counter operations achieve target performance (< 50ns/op)
- Parallel operations show good scaling (25.2ns → 8.5ns with 8 cores)
- Zero allocations confirmed for metric updates
- Registry exports have reasonable cost with minimal allocations

### Red Flags

**Performance Regression:**
- Any metric update showing > 0 allocs/op
- Counter operations > 100ns/op
- Histogram operations > 500ns/op
- Memory growth in long-running tests

**Concurrency Issues:**
- Parallel performance worse than sequential
- Lock contention in block profiles
- Goroutine leaks in stress tests
- Memory leaks across registry lifecycle

## Continuous Performance Monitoring

### CI Integration

These benchmarks run in CI with performance regression detection:
- 20% performance regression = build failure
- Memory allocation increase = immediate alert  
- Goroutine leak detection in stress tests
- Comparison benchmark validation

### Historical Tracking

Benchmark results are stored for trend analysis:
- Performance regression detection across releases
- Optimization impact measurement
- Performance characteristic documentation
- Comparison benchmark scoreboard

## Contributing Performance Tests

### Adding New Benchmarks

When adding benchmarks, follow these patterns:

**1. Real-World Scenarios**
```go
// BAD - toy example
func BenchmarkCounter(b *testing.B) {
    counter.Inc()
}

// GOOD - realistic usage
func BenchmarkHTTPRequestTracking(b *testing.B) {
    // Full request lifecycle with multiple metrics
}
```

**2. Realistic Data Distributions**
```go
// Use realistic data patterns
for pb.Next() {
    value := generateRealisticValue() // Not just rand.Float64()
    histogram.Observe(value)
}
```

**3. Proper Benchmark Setup**
```go
func BenchmarkOperation(b *testing.B) {
    // Setup outside timing
    registry := metricz.New()
    metric := registry.Counter(TestKey)
    
    b.ResetTimer()  // Start timing here
    b.ReportAllocs() // Track allocations
    
    for i := 0; i < b.N; i++ {
        // Only measured operations here
    }
}
```

### Benchmark Standards

**Requirements:**
- All benchmarks must reflect realistic usage
- Zero allocation claims must be verified
- Parallel versions for concurrency testing  
- Memory pressure testing where relevant
- Proper timer management (ResetTimer, StopTimer)

**Documentation:**
- Clear benchmark purpose and scenario
- Performance expectations documented
- Failure conditions defined
- Integration with existing benchmark suite

## Performance Promise

The metricz library promises:
1. **Zero allocation updates** after metric creation
2. **Lock-free reads** for all metric values  
3. **Minimal contention** with atomic operations
4. **Constant time lookup** by key
5. **Predictable memory usage** with no cardinality explosion

These benchmarks validate every claim with realistic workloads and honest measurement. No bullshit, no vanity metrics - just real performance data for real systems.

---

**CRASH Performance Engineering**  
*Real performance measurement for real systems*

For questions about benchmark methodology or performance issues, check the `.remarks/*/performance/` directories for detailed analysis and recommendations.