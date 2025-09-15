# API Gateway Metrics Example

## Black Friday Meltdown Scenario

Experience a real production incident and see how metrics save the day:

```bash
# Run the Black Friday scenario
go run . -scenario blackfriday

# Or use the interactive script
./run_scenarios.sh
```

### The Story

It's Black Friday 2023. Your e-commerce platform is handling massive traffic. Suddenly, the payment service starts degrading. Without circuit breakers, aggressive retries create a cascade failure that brings down the entire platform.

Watch as:
1. **09:15 AM** - Traffic ramps up normally
2. **09:45 AM** - Payment latency starts climbing (500ms → 1,200ms)
3. **10:15 AM** - Full meltdown! P99 latency hits 4,800ms
4. **10:30 AM** - Engineering enables circuit breaker
5. **10:45 AM** - Service recovers, revenue saved

The scenario demonstrates:
- How aggressive retries make problems worse
- Connection pool exhaustion patterns
- Circuit breaker patterns in action
- Real-time metrics revealing the problem
- P99 latency monitoring saving Black Friday

## The Problem

You're building an API gateway that proxies requests to multiple backend services. You need visibility into:
- Request latencies per endpoint
- Error rates by status code
- Concurrent request counts
- Backend service health

Without proper metrics, you're flying blind. When users complain about slow responses, you have no data. When a service degrades, you find out from angry customers.

## The Discovery Journey

### Phase 1: Basic Counting (What We Started With)

```go
var requestCount int64
var errorCount int64

func handleRequest(w http.ResponseWriter, r *http.Request) {
    atomic.AddInt64(&requestCount, 1)
    
    // ... handle request ...
    
    if err != nil {
        atomic.AddInt64(&errorCount, 1)
    }
}
```

Problems emerged immediately:
- No visibility per endpoint
- No latency tracking
- Global state made testing impossible
- Race conditions in more complex scenarios

### Phase 2: Adding Structure (First Attempt)

```go
type Metrics struct {
    mu           sync.RWMutex
    requests     map[string]int64
    errors       map[string]int64
    latencies    map[string][]float64
}
```

This created new problems:
- Memory grew unbounded with latency arrays
- Lock contention under load
- No percentile calculations
- Still hard to test in isolation

### Phase 3: Discovery of metricz

The metricz library solves these problems:
- **Registry isolation**: Each service gets its own metrics registry
- **Proper histograms**: Bucketed latency tracking without unbounded memory
- **Atomic operations**: No lock contention for counters/gauges
- **Testing support**: Clean reset between tests

## The Solution

This example demonstrates a production-ready API gateway with comprehensive metrics:

1. **Request metrics per endpoint**
2. **Latency histograms with percentiles**
3. **Error tracking by status code**
4. **Concurrent request gauges**
5. **Backend health monitoring**
6. **Prometheus export support**

## Performance Impact

Per request overhead (measured on AMD Ryzen 5 3600X):
- Counter increment: ~5ns (request counting)
- Timer recording: ~200ns (latency tracking)
- Gauge operations: ~5ns (concurrent request tracking)
- **Total overhead: ~210ns per request (0.000210ms)**

At 10,000 RPS: 0.002 seconds CPU/second (0.2% CPU overhead)
At 50,000 RPS: 0.010 seconds CPU/second (1.0% CPU overhead)

The HTTP tracking benchmark shows realistic gateway performance:
```bash
go test -bench=BenchmarkHTTPRequestTracking -benchmem ../../testing/benchmarks/
```

## Running the Example

```bash
# Run the API gateway
go run main.go

# In another terminal, generate some traffic
./generate_traffic.sh

# View metrics
curl http://localhost:8080/metrics
```

## What You'll Learn

1. **Registry Patterns**: How to structure metrics in a service
2. **Histogram Usage**: Tracking latencies with proper buckets
3. **Label Patterns**: Using formatted keys for dimensional data
4. **Export Integration**: Prometheus-compatible metrics endpoint
5. **Testing Strategies**: Isolated metrics testing

## Implementation Phases

### Phase 1: Basic Gateway
- Simple proxy with request counting
- Error tracking
- Basic latency measurement

### Phase 2: Enhanced Observability
- Per-endpoint metrics
- Status code breakdown
- Backend health tracking

### Phase 3: Production Features
- Circuit breaker integration
- Rate limiting metrics
- Custom bucket configurations
- Grafana dashboard example

## Key Insights

1. **Isolation Matters**: Global metrics make testing a nightmare. Registry isolation fixes this.

2. **Histograms Over Arrays**: Never store raw latencies. Use histograms with defined buckets.

3. **Labels as Keys**: Instead of complex label systems, use formatted keys: `requests_/api/users_GET`

4. **Atomic Operations**: Counters and gauges use atomic operations - no mutex needed for basic operations.

5. **Reset for Testing**: The registry Reset() method enables clean test isolation.

## Files in This Example

- `main.go` - Complete API gateway implementation
- `gateway.go` - Gateway logic with metrics integration  
- `backends.go` - Mock backend services
- `gateway_test.go` - Comprehensive testing patterns
- `generate_traffic.sh` - Traffic generation script
- `dashboard.json` - Grafana dashboard configuration