# Reliability Tests

This directory contains stress tests and edge case validation to ensure the metricz library remains stable under extreme conditions and handles boundary cases correctly.

## Test Coverage

### Concurrency Stress Testing (`concurrency_stress_test.go`)
- **CAS loop starvation**: Atomic compare-and-swap under extreme contention
- **Reset under load**: Registry reset while concurrent operations are running
- **Concurrent metric creation**: Thousands of goroutines creating unique metrics
- **Chaos operations**: Random operations to find race conditions
- **High-frequency timers**: Timer performance under rapid recording
- **Reader/writer interference**: Ensuring readers don't block writers

**Stress levels:**
- **CI mode** (default): Quick validation with moderate load
- **Stress mode** (`METRICZ_STRESS_TEST=true`): Heavy load testing

**Key scenarios tested:**
- 1,000 workers performing 10,000 operations on shared counter
- Registry reset every 10ms while 100 goroutines read/write metrics
- 50,000 concurrent metric creations
- Random operations with 100 chaos goroutines
- Timer handling 100,000+ recordings per second

### Memory Pressure Testing (`memory_pressure_test.go`)
- **Unbounded metric creation**: Memory usage with 100,000+ unique metrics
- **Large histogram buckets**: Performance with 1,000 bucket histograms
- **Maximum key lengths**: 200-character metric names
- **Reset memory leaks**: Ensuring Reset() properly frees memory
- **Histogram observation memory**: Constant memory with unlimited observations

**Memory validation:**
- Tracks memory growth per metric created
- Alerts when memory per metric exceeds 1KB
- Verifies Reset() reduces memory usage
- Confirms histograms don't grow with observations

**Key scenarios tested:**
- Creating 100,000 unique metrics and measuring memory efficiency
- Histograms with 1,000 buckets handling 100,000 observations
- Maximum-length metric names (200 chars) memory impact
- Multiple create/reset cycles to detect leaks

### Overflow Handling (`overflow_test.go`)
- **Histogram overflow buckets**: Values exceeding all bucket bounds
- **Float64 precision limits**: Counter behavior near 2^53 boundary
- **NaN/Infinity rejection**: Invalid value handling
- **Negative value handling**: Counter rejection, gauge acceptance
- **Timer precision**: Nanosecond to day-long duration handling
- **Empty/single bucket histograms**: Edge case bucket configurations

**Boundary conditions tested:**
- Values exceeding histogram bucket ranges → overflow bucket
- Counter increments at float64 precision limit (2^53)
- NaN and ±Infinity value rejection across all metric types
- Negative counter values (rejected) vs negative gauge values (accepted)
- Timer durations from 1 nanosecond to 24 hours
- Histograms with zero buckets or single bucket

**Validation approach:**
- **Data integrity**: No observations lost, all values accounted for
- **Overflow buckets**: Values exceeding ranges properly captured
- **Input validation**: Invalid inputs rejected without corruption
- **Precision documentation**: Expected float64 limitations documented

## Running Reliability Tests

```bash
# Quick CI validation (default)
go test ./testing/reliability/

# Heavy stress testing  
METRICZ_STRESS_TEST=true go test ./testing/reliability/ -timeout=30s

# Run with race detection
go test ./testing/reliability/ -race

# Specific test categories
go test ./testing/reliability/ -run TestCASLoopStarvation
go test ./testing/reliability/ -run TestUnboundedMetricCreation  
go test ./testing/reliability/ -run TestNaNAndInfinity

# Memory profiling during tests
go test ./testing/reliability/ -memprofile=mem.prof
```

## Stress Test Configuration

The tests automatically adjust load based on environment:

### CI Mode (Default)
- **Workers**: 10-50 goroutines
- **Operations**: 100-1,000 per worker  
- **Duration**: 100-500 milliseconds
- **Metrics**: 1,000-10,000 unique metrics

### Stress Mode (`METRICZ_STRESS_TEST=true`)
- **Workers**: 100-1,000 goroutines
- **Operations**: 1,000-10,000 per worker
- **Duration**: 1-10 seconds  
- **Metrics**: 10,000-100,000 unique metrics

## What These Tests Verify

### Concurrency Safety
- No race conditions under extreme concurrent load
- Atomic operations never starve or deadlock
- Registry operations remain consistent during reset
- Readers and writers don't interfere with each other

### Memory Efficiency  
- Memory usage scales linearly with metric count
- No memory leaks during metric lifecycle
- Reset operations properly free allocated memory
- Large histograms remain memory-efficient

### Data Integrity
- No metric values lost under stress
- Overflow values properly captured in histograms
- Invalid inputs (NaN, Infinity) properly rejected
- Precision limitations documented and handled gracefully

### Performance Resilience
- Performance remains acceptable under heavy load
- Memory pressure doesn't cause failures
- High-frequency operations maintain accuracy
- System remains responsive during stress

## Expected Behavior

These tests document **expected limitations** of float64-based metrics:

- **Precision loss**: Counter increments have no effect near 2^53
- **Input validation**: NaN and Infinity values are rejected
- **Overflow handling**: Histogram overflow buckets prevent data loss
- **Memory scaling**: Approximately 100-200 bytes per metric

The tests verify the library handles these limitations gracefully without corruption or crashes.