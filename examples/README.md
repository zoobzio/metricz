# Metricz Examples

This directory contains comprehensive examples demonstrating metricz usage patterns following the **wrong-then-right teaching methodology**. Each example shows the stark contrast between uninstrumented code (which hides problems) and instrumented code (which reveals and solves problems).

## Example Structure

All examples follow the same proven pattern:

### Section 1: Naive Implementation (No Metrics)
- Shows how code typically looks without instrumentation
- Appears functional but hides critical problems
- No visibility into performance, failures, or system health

### Section 2: Instrumented Implementation (With Comprehensive Metrics)
- Demonstrates proper metricz integration
- Provides complete observability into system behavior
- Enables data-driven optimization and debugging

### Section 3: Load Simulation and Problem Injection
- Realistic workload generators with configurable failure modes
- Simulates real-world conditions that expose hidden issues
- Demonstrates failure patterns that metrics reveal

### Section 4: Automated Problem Detection
- Built-in analysis engines that detect patterns automatically
- Provides actionable recommendations based on metric data
- Proves the value of instrumentation through concrete examples

### Section 5: Side-by-Side Comparison
- Runs both implementations under identical conditions
- Shows exactly what visibility is gained through metrics
- Demonstrates problem detection and resolution capabilities

## Available Examples

### API Gateway (`api-gateway/`)
**280 lines - Comprehensive API gateway metrics**

Shows how metrics transform an API gateway from a black box into a transparent, debuggable system:

- **Request/Response Tracking**: Visibility into traffic patterns and response codes
- **Service Health Monitoring**: Backend service performance and availability tracking  
- **Circuit Breaker Integration**: Failure rate monitoring and protection mechanisms
- **Latency Analysis**: Total vs backend latency with overhead calculation
- **Error Categorization**: Detailed error classification and impact analysis

**Key Learning**: Without metrics, gateway problems remain invisible until they become outages. With metrics, problems are detected and resolved proactively.

**Run**: `cd api-gateway && go run main.go`

### Batch Processor (`batch-processor/`)
**987 lines - Complete batch processing instrumentation**

Demonstrates how metrics reveal bottlenecks and failures in batch processing systems:

- **Stage-by-Stage Visibility**: Tracking processing through validation, transformation, persistence, cleanup
- **Item Type Analysis**: Performance patterns by data type and size
- **Retry Intelligence**: Success rates and failure pattern detection
- **Worker Load Balancing**: Utilization tracking across processing workers
- **Queue Management**: Depth monitoring and capacity planning insights
- **Resource Utilization**: Memory pressure and system resource tracking

**Key Learning**: Batch processing mysteries become diagnosable problems when proper metrics expose what's actually happening.

**Run**: `cd batch-processor && go run main.go`

### Service Mesh (`service-mesh/`)
**[Legacy Example]** - Original version, does not follow wrong-then-right pattern

### Worker Pool (`worker-pool/`)  
**[Legacy Example]** - Original version, does not follow wrong-then-right pattern

## Common Patterns Across Examples

### 1. Registry Isolation
Every example creates its own registry instance:
```go
registry := metricz.New()
```
This ensures complete metric isolation and enables clean testing.

### 2. Metric Organization
Metrics are organized by purpose:
- **Counters**: For cumulative values (requests, errors, successes)
- **Gauges**: For current values (queue depth, active workers, progress)
- **Timers**: For duration tracking (latencies, processing times)
- **Histograms**: For distributions (request sizes, response times)

### 3. Key Naming Conventions
Using Key type constants for compile-time safety:
```go
// Define keys as constants - NEVER create dynamically
const (
    RequestsUserServiceGET    metricz.Key = "requests_user_service_GET"
    LatencyOrderService       metricz.Key = "latency_order_service"
    QueueDepthHighPriority    metricz.Key = "queue_depth_high_priority"
)

// Use the constants - compile-time verified
registry.Counter(RequestsUserServiceGET)
registry.Timer(LatencyOrderService)
registry.Gauge(QueueDepthHighPriority)
```

### 4. Testing Strategies
Each example includes comprehensive tests demonstrating:
- Isolated testing with registry reset
- Concurrent operation verification
- Metric accuracy validation
- Failure scenario testing

### 5. Graceful Shutdown
All examples handle shutdown gracefully:
```go
shutdown := make(chan os.Signal, 1)
signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
<-shutdown
// Capture final metrics before exit
```

## Learning Path

1. **Start with API Gateway** - Simplest integration patterns
2. **Move to Worker Pool** - Concurrent metrics and gauges
3. **Try Batch Processor** - Progress tracking and checkpointing
4. **Explore Service Mesh** - Advanced patterns like circuit breakers

## Integration with Monitoring Systems

### Prometheus
All examples include Prometheus-compatible export endpoints:
```go
func MetricsHandler(registry metricz.Registry) http.HandlerFunc {
    // Export counters, gauges, histograms, timers
}
```

### Custom Exporters
Examples show how to build custom metric exporters for any monitoring system.

## Performance Considerations

### Atomic Operations
Counters and gauges use atomic operations for lock-free updates:
```go
counter.Inc()  // Atomic increment
gauge.Set(42)  // Atomic set
```

### Histogram Buckets
Choose buckets based on your SLAs:
```go
// API latencies: milliseconds
buckets := []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000}

// Batch processing: seconds
buckets := []float64{1, 5, 10, 30, 60, 120, 300, 600}
```

### Memory Management
- Histograms use fixed buckets (bounded memory)
- No unbounded arrays or maps
- Registry reset cleans all metrics

## Running All Examples

```bash
# Run all examples sequentially
for dir in api-gateway worker-pool batch-processor service-mesh; do
    echo "Running $dir example..."
    (cd $dir && go run . &)
    sleep 30
    pkill -f "go run"
done
```

## Testing All Examples

```bash
# Test all examples
for dir in api-gateway worker-pool batch-processor service-mesh; do
    echo "Testing $dir..."
    (cd $dir && go test -v)
done
```

## Contributing New Examples

When adding new examples:
1. Create a clear problem statement in README
2. Show the journey from naive solution to metricz
3. Include runnable code with realistic scenarios
4. Add comprehensive tests
5. Document key insights and patterns

## Questions or Issues?

Each example is self-contained and runnable. If you encounter issues:
1. Ensure you have Go 1.21+ installed
2. Run `go mod tidy` in the example directory
3. Check the example-specific README for details