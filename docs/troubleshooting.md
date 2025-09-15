# Troubleshooting

## Values Not Updating

### Counter Not Incrementing

**Check for negative values**:
```go
counter := registry.Counter("requests_total")
counter.Add(-1)        // Silently ignored
counter.Add(math.NaN()) // Silently ignored

// Solution: Validate inputs
if delta > 0 && !math.IsNaN(delta) && !math.IsInf(delta, 0) {
    counter.Add(delta)
}
```

**Check for wrong metric type**:
```go
// This creates a counter
metric := registry.Counter("memory_usage")
metric.Add(-100) // Ignored - counters don't decrease

// Use gauge for bidirectional values  
gauge := registry.Gauge("memory_usage")
gauge.Add(-100) // Works - gauges can decrease
```

### Gauge Not Changing

**Check for invalid values**:
```go
gauge := registry.Gauge("temperature")
gauge.Set(math.NaN()) // Silently ignored
gauge.Set(math.Inf(1)) // Silently ignored

// Solution: Validate before setting
if !math.IsNaN(value) && !math.IsInf(value, 0) {
    gauge.Set(value)
}
```

### Histogram Not Recording

**Check observation values**:
```go  
histogram := registry.Histogram("response_time", []float64{0.1, 0.5, 1.0})
histogram.Observe(-0.1)     // Ignored - negative
histogram.Observe(math.NaN()) // Ignored - NaN

// Solution: Validate observations
if value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0) {
    histogram.Observe(value)
}
```

**Check bucket configuration**:
```go
// Too few buckets might not capture your data
histogram := registry.Histogram("latency", []float64{1.0}) // Only 0-1s and 1s+
histogram.Observe(0.5) // Goes in first bucket
histogram.Observe(2.0) // Goes in +Inf bucket

// Solution: Better bucket distribution
buckets := []float64{0.1, 0.25, 0.5, 1.0, 2.5, 5.0}
histogram := registry.Histogram("latency", buckets)
```

## High Memory Usage

### Unbounded Metric Keys

**Problem**: Creating metrics with dynamic keys
```go
// DON'T: Creates unlimited metrics
for _, user := range users {
    counter := registry.Counter(fmt.Sprintf("user_%s_requests", user.ID))
    counter.Inc()
}
// Memory usage grows with unique users
```

**Solution**: Use labels in external systems or bounded key sets
```go
// DO: Fixed set of metrics
userRequests := registry.Counter("user_requests_total")
adminRequests := registry.Counter("admin_requests_total")  

// Or use external labeling (Prometheus)
// user_requests_total{user_id="123"} 1
```

### Memory Leak Detection

**Check metric count growth**:
```go
func debugMetricCount(registry *metricz.Registry) {
    values := registry.Values()
    log.Printf("Current metric count: %d", len(values))
    
    for name := range values {
        log.Printf("Metric: %s", name)
    }
}

// Call periodically to detect unbounded growth
```

## Slow Exports

### Too Many Metrics

**Problem**: Thousands of metrics slow export
```go
// Creating 10,000 counters
for i := 0; i < 10000; i++ {
    counter := registry.Counter(fmt.Sprintf("metric_%d", i))
}
// Export takes seconds
```

**Solution**: Reduce metric count or use sampling
```go
// Aggregate similar metrics  
requests := registry.Counter("requests_total")
errors := registry.Counter("errors_total")

// Use histograms for distributions instead of many gauges
response_times := registry.Histogram("response_time", buckets)
```

### Export Optimization

**Buffer exports**:
```go
func efficientExport(registry *metricz.Registry, w http.ResponseWriter) {
    buf := &bytes.Buffer{}
    registry.Export(buf) // Export to buffer first
    
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
    buf.WriteTo(w)
}
```

## Incorrect Values

### Timer Issues

**Not calling stop function**:
```go
// WRONG: Timer not stopped
stop := timer.Start()
doWork()
// stop() never called - no measurement recorded

// CORRECT: Always call stop
stop := timer.Start()
defer stop() // Ensures stop is called
doWork()
```

**Multiple start calls**:
```go
// WRONG: Multiple starts without stops  
stop1 := timer.Start()
stop2 := timer.Start() // Overwrites first timer
stop1() // May record incorrect duration
stop2() // Records from second start

// CORRECT: One start per timing
func timedOperation() {
    stop := timer.Start()
    defer stop()
    // ... do work
}
```

### Histogram Bucket Issues

**Bucket boundaries don't match data**:
```go
// Response times are typically 1-100ms 
buckets := []float64{1, 5, 10} // In seconds - too large!
histogram := registry.Histogram("response_time", buckets)
histogram.Observe(0.05) // 50ms - all go in first bucket

// CORRECT: Match bucket scale to data scale
buckets := []float64{0.01, 0.05, 0.1, 0.5, 1.0} // Seconds
histogram.Observe(0.05) // Correctly distributed
```

**Exponential vs Linear buckets**:
```go
// Linear buckets for uniform distribution
linear := []float64{0.1, 0.2, 0.3, 0.4, 0.5}

// Exponential buckets for skewed distribution  
exponential := []float64{0.01, 0.02, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0}
```

## Integration Issues

### Middleware Not Recording

**Handler ordering**:
```go
// WRONG: Metrics after other middleware
handler := loggingMiddleware(metricsMiddleware(actualHandler))
// Logging might interfere with metrics

// CORRECT: Metrics first
handler := metricsMiddleware(loggingMiddleware(actualHandler))
```

**Panic recovery**:
```go
func metricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requests.Inc()
        stop := timer.Start()
        
        // Handle panics to ensure timer stops
        defer func() {
            if r := recover(); r != nil {
                errors.Inc()
                stop() // Still record timing
                panic(r) // Re-panic after recording
            }
        }()
        
        next.ServeHTTP(w, r)
        stop()
    })
}
```

### Export Endpoint Issues

**Wrong content type**:
```go
// WRONG: Missing or incorrect content type
func badMetricsHandler(w http.ResponseWriter, r *http.Request) {
    registry.Export(w) // Default content type
}

// CORRECT: Proper Prometheus content type
func goodMetricsHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
    registry.Export(w)
}
```

**Export errors not handled**:
```go
// WRONG: No error handling
func fragileHandler(w http.ResponseWriter, r *http.Request) {
    registry.Export(w) // Could fail silently
}

// CORRECT: Handle export errors
func robustHandler(w http.ResponseWriter, r *http.Request) {
    buf := &bytes.Buffer{}
    if err := registry.Export(buf); err != nil {
        http.Error(w, "Export failed", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    buf.WriteTo(w)
}
```

## Debugging Tools

### Value Inspection
```go
func debugMetrics(registry *metricz.Registry) {
    values := registry.Values()
    
    for name, value := range values {
        switch v := value.(type) {
        case map[string]interface{}: // Histogram/Timer
            fmt.Printf("%s: count=%v, sum=%v\n", name, v["count"], v["sum"])
        case float64: // Counter/Gauge
            fmt.Printf("%s: %.2f\n", name, v)
        }
    }
}
```

### Export Verification
```go
func verifyExport(registry *metricz.Registry) {
    buf := &bytes.Buffer{}
    registry.Export(buf)
    
    output := buf.String()
    lines := strings.Split(output, "\n")
    
    fmt.Printf("Export contains %d lines\n", len(lines))
    
    for _, line := range lines {
        if strings.HasPrefix(line, "# HELP") {
            fmt.Printf("Help: %s\n", line)
        } else if strings.Contains(line, " ") && !strings.HasPrefix(line, "#") {
            fmt.Printf("Metric: %s\n", line)
        }
    }
}
```