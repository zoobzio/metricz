# Integration Tests

This directory contains integration tests that validate real-world usage patterns and cross-component interactions in the metricz library.

## Test Coverage

### Aggregation Patterns (`aggregation_test.go`)
- **Multi-worker aggregation**: Combining metrics from multiple worker registries
- **Percentile calculations**: Computing percentiles from histogram buckets  
- **Rolling window metrics**: Time-based sliding window rate calculations
- **Multi-dimensional aggregation**: Grouping metrics by method, status code, endpoint
- **Cascading aggregation**: Three-tier aggregation (instances → services → cluster)

**Key scenarios tested:**
- Worker task processing with error rates
- Response time percentile computation (P50, P95, P99)
- Request rate monitoring over time windows
- Error rate calculations per endpoint
- Service health scoring across multiple tiers

### Export Patterns (`export_patterns_test.go`)
- **Prometheus format**: Standard metrics exposition format
- **JSON export**: Structured metric export with metadata
- **Streaming export**: Real-time metric updates via channels
- **Custom exporters**: StatsD format implementation
- **Delta exports**: Calculating metric changes over time
- **Batch exports**: Multi-service metric aggregation
- **Filtered exports**: Selective metric exposure (public vs private)
- **Sorted exports**: Deterministic metric ordering
- **Compressed exports**: Space-efficient export formats

**Key scenarios tested:**
- Prometheus histogram bucket formatting
- JSON serialization/deserialization round-trips
- Real-time metric streaming
- Rate calculation from metric deltas
- Service-specific metric batching

### Isolation Testing (`isolation_test.go`)
- **Registry isolation**: Complete independence between registry instances
- **Concurrent instances**: Multiple registries operating simultaneously
- **Reset isolation**: Reset operations affecting only target registry
- **Lifecycle independence**: Registry creation and destruction
- **Reader isolation**: Registry readers see only their metrics
- **Parallel test safety**: Test functions can run concurrently
- **No global state**: Verification of zero shared state

**Key scenarios tested:**
- 50 concurrent registries with independent operations
- Registry cleanup without affecting others
- Cross-contamination prevention
- Memory isolation verification

### Service Lifecycle (`service_lifecycle_test.go`)
- **Service metrics patterns**: Complete service instrumentation
- **Graceful shutdown**: Metrics export during shutdown
- **Multi-service isolation**: Independent service metrics
- **Recovery tracking**: Service health and recovery patterns
- **Long-running processes**: Batch processing metrics
- **Dependency tracking**: External service call monitoring

**Key scenarios tested:**
- HTTP service with cache hits/misses, database queries
- Service shutdown with final metrics export
- Health check cycles with failure/recovery tracking
- Batch processing with progress monitoring
- External dependency availability tracking

### Black Friday Scenario (`black_friday_test.go`)
- **Circuit breaker pattern**: Preventing cascade failures under load
- **Service degradation**: High latency and error rate handling
- **Retry storm prevention**: Circuit breaker protecting downstream services
- **Recovery testing**: Service restoration after degradation
- **Incident metrics**: Tracking circuit breaker trips and saved requests

**Real-world scenario:**
Simulates payment service degradation during high traffic, verifying that circuit breakers prevent retry storms while collecting incident metrics.

## Running Integration Tests

```bash
# Run all integration tests
go test ./testing/integration/

# Run specific test categories
go test ./testing/integration/ -run TestMetricAggregation
go test ./testing/integration/ -run TestPrometheusExport
go test ./testing/integration/ -run TestCompleteInstanceIsolation
go test ./testing/integration/ -run TestBlackFridayScenario

# Run with verbose output for detailed scenarios
go test ./testing/integration/ -v
```

## Test Patterns

These tests demonstrate the **correct patterns** for production usage:

1. **Registry per service**: Each service gets its own isolated registry
2. **Static keys**: Use `metricz.Key` type with constant definitions
3. **Metric aggregation**: Combine metrics from multiple sources
4. **Export integration**: Multiple export formats and patterns
5. **Circuit breaker patterns**: Resilience patterns with metrics
6. **Health monitoring**: Service lifecycle and dependency tracking

## What These Tests Verify

- Metrics correctly aggregate across multiple data sources
- Export formats produce valid, parseable output
- Services remain isolated from each other's metrics
- Real-world resilience patterns work as expected
- Service lifecycle events are properly tracked
- Performance remains acceptable under realistic load