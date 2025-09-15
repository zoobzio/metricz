# API Reference

## Registry

### New()
Creates isolated metrics registry.
```go
registry := metricz.New()
// Returns: *Registry
```

### Counter(key string)  
Creates or retrieves counter metric.
```go
counter := registry.Counter("requests_total")
// Returns: Counter interface
```

### Gauge(key string)
Creates or retrieves gauge metric.
```go
gauge := registry.Gauge("memory_bytes")  
// Returns: Gauge interface
```

### Histogram(key string, buckets []float64)
Creates or retrieves histogram metric.
```go
histogram := registry.Histogram("response_time", []float64{0.1, 0.5, 1.0})
// Returns: Histogram interface
```

### Timer(key string, buckets []float64)
Creates or retrieves timer metric.
```go
timer := registry.Timer("db_query_duration", []float64{0.01, 0.1, 1.0})
// Returns: Timer interface
```

### Export(w io.Writer)
Writes metrics in Prometheus format.
```go
registry.Export(w)
// Output: Prometheus text format
```

### ExportJSON(w io.Writer)  
Writes metrics in JSON format.
```go
registry.ExportJSON(w)
// Output: JSON object with metric values
```

### Values()
Returns current metric values.
```go
values := registry.Values()
// Returns: map[string]interface{}
```

## Counter

Monotonic counter - values only increase.

### Inc()
Increments counter by 1.
```go
counter.Inc()
// counter.Value() increases by 1
```

### Add(delta float64)
Adds value to counter.
```go
counter.Add(5.5)
// counter.Value() increases by 5.5
```
**Validation**: Ignores negative values, NaN, infinity.

### Value() 
Returns current counter value.
```go
value := counter.Value()
// Returns: float64
```

## Gauge

Bidirectional gauge - values can increase or decrease.

### Set(value float64)
Sets gauge to specific value.
```go
gauge.Set(100)  
// gauge.Value() == 100
```

### Add(delta float64)
Adds to current value.
```go
gauge.Add(25)
// gauge.Value() increases by 25
```

### Sub(delta float64)  
Subtracts from current value.
```go
gauge.Sub(10)
// gauge.Value() decreases by 10
```

### Inc()
Increments gauge by 1.
```go
gauge.Inc()
// gauge.Value() increases by 1
```

### Dec()
Decrements gauge by 1.
```go
gauge.Dec()  
// gauge.Value() decreases by 1
```

### Value()
Returns current gauge value.
```go
value := gauge.Value()
// Returns: float64
```

## Histogram

Distribution tracking with configurable buckets.

### Observe(value float64)
Records observation in histogram.
```go
histogram.Observe(0.25)
// Increments appropriate bucket counters
```

### Count()
Returns total number of observations.
```go
count := histogram.Count()  
// Returns: uint64
```

### Sum()
Returns sum of all observed values.
```go
sum := histogram.Sum()
// Returns: float64
```

### Buckets()
Returns configured bucket boundaries.
```go
buckets := histogram.Buckets()
// Returns: []float64
```

### BucketCounts()
Returns observation counts per bucket.
```go
counts := histogram.BucketCounts() 
// Returns: []uint64 (same length as Buckets())
```

## Timer

Duration measurement with histogram distribution.

### Start()
Begins timing operation.
```go
stop := timer.Start()
// Returns: function to stop timing
```

### Stop Function
Stops timing and records duration.
```go
stop := timer.Start()
// ... do work ...  
stop()
// Records elapsed duration in histogram
```

### Observe(duration time.Duration)
Manually records duration.
```go
timer.Observe(250 * time.Millisecond)
// Records 0.25 seconds in histogram
```

### Count()
Returns total number of timing observations.
```go
count := timer.Count()
// Returns: uint64
```

### Sum()  
Returns total duration of all observations.
```go
total := timer.Sum()
// Returns: float64 (seconds)
```

### Buckets()
Returns configured duration buckets.
```go
buckets := timer.Buckets()
// Returns: []float64 (seconds)
```

### BucketCounts()
Returns timing counts per bucket.
```go  
counts := timer.BucketCounts()
// Returns: []uint64
```

## Data Types

### Key Type
Metric identifier - use string literals or constants.
```go
const RequestsTotal = "requests_total"
counter := registry.Counter(RequestsTotal)
```

### Bucket Configuration
Histogram and timer bucket boundaries.
```go
// Response time buckets (seconds)
latencyBuckets := []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0}

// Size buckets (bytes)  
sizeBuckets := []float64{100, 1000, 10000, 100000, 1000000}
```

## Thread Safety

All operations are thread-safe:
```go
counter := registry.Counter("concurrent_ops")

// Safe from multiple goroutines
go func() { counter.Inc() }()
go func() { counter.Add(5) }()
go func() { fmt.Println(counter.Value()) }()
```

## Validation Behavior

Invalid inputs are silently ignored:

**Counter**: Rejects negative values, NaN, infinity
```go
counter.Add(-1)        // Ignored
counter.Add(math.NaN()) // Ignored  
```

**Gauge**: Rejects NaN, infinity (allows negative)
```go
gauge.Set(math.NaN()) // Ignored
gauge.Set(-100)       // Accepted
```

**Histogram/Timer**: Rejects negative values, NaN, infinity
```go
histogram.Observe(-1)        // Ignored
timer.Observe(-time.Second)  // Ignored
```

## Export Formats

### Prometheus
Standard metrics format for monitoring systems.
```
# HELP requests_total Total HTTP requests
# TYPE requests_total counter
requests_total 42

# HELP response_time Response time histogram  
# TYPE response_time histogram
response_time_bucket{le="0.1"} 10
response_time_bucket{le="0.5"} 25
response_time_bucket{le="+Inf"} 30
response_time_count 30
response_time_sum 8.5
```

### JSON
Structured format for programmatic access.
```json
{
  "requests_total": {
    "type": "counter", 
    "value": 42
  },
  "response_time": {
    "type": "histogram",
    "count": 30,
    "sum": 8.5,
    "buckets": {"0.1": 10, "0.5": 25, "+Inf": 30}
  }
}
```