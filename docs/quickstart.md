# Quick Start

## 30-Second Success

Create and use a counter:

```go
package main

import (
    "fmt"
    "github.com/zoobzio/metricz"
)

func main() {
    registry := metricz.New()
    counter := registry.Counter("requests")
    counter.Inc()
    fmt.Printf("Requests: %.0f", counter.Value()) // Requests: 1
}
```

That's it. Your first metric is working.

## 5-Minute Mastery

All four metric types:

```go
registry := metricz.New()

// Counter - values only go up
requests := registry.Counter("requests_total")
requests.Inc()        // Add 1
requests.Add(5)       // Add 5
// requests.Value() == 6

// Gauge - values go up and down  
memory := registry.Gauge("memory_bytes")
memory.Set(1024)      // Set value
memory.Add(256)       // Add to current
memory.Sub(128)       // Subtract from current
// memory.Value() == 1152

// Histogram - measure distributions
latency := registry.Histogram("response_time", []float64{0.1, 0.5, 1.0})
latency.Observe(0.3)  // Record observation
latency.Observe(0.7)
// latency.Count() == 2, latency.Sum() == 1.0

// Timer - measure durations
timer := registry.Timer("db_query_duration", []float64{0.01, 0.1, 1.0})
stop := timer.Start()
// ... do work ...
stop()  // Records duration automatically
```

## Production Basics

### HTTP Server Integration

```go
func main() {
    registry := metricz.New()
    requests := registry.Counter("http_requests_total")
    
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        requests.Inc()
        w.Write([]byte("Hello World"))
    })
    
    // Export metrics endpoint
    http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
        registry.Export(w)
    })
    
    http.ListenAndServe(":8080", nil)
}
```

### Background Worker

```go
func worker(registry *metricz.Registry) {
    processed := registry.Counter("jobs_processed")
    errors := registry.Counter("jobs_failed") 
    duration := registry.Timer("job_duration", []float64{0.1, 0.5, 1.0, 5.0})
    
    for job := range jobQueue {
        stop := duration.Start()
        
        if err := processJob(job); err != nil {
            errors.Inc()
        } else {
            processed.Inc()
        }
        
        stop()
    }
}
```

### Export Formats

```go
// Prometheus format (default)
registry.Export(w) 

// JSON format
registry.ExportJSON(w)

// Get raw values
metrics := registry.Values()
fmt.Printf("Current metrics: %+v", metrics)
```

## Key Concepts

**Registry**: Container for all your metrics. Create one per application:
```go
registry := metricz.New()
```

**Thread Safety**: All operations are thread-safe. Use from multiple goroutines:
```go
counter := registry.Counter("concurrent_operations")
go func() { counter.Inc() }() // Safe
go func() { counter.Inc() }() // Safe
```

**Key Types**: Use strings for metric names. Constants recommended for consistency:
```go
const RequestsTotal = "requests_total"
counter := registry.Counter(RequestsTotal)
```

**Validation**: Invalid inputs are ignored (negative counter values, NaN, infinity):
```go
counter.Add(-1)        // Ignored
counter.Add(math.NaN()) // Ignored  
// Counter value unchanged
```

## Next Steps

- **Common Patterns**: See [patterns.md](patterns.md) for HTTP middleware, export patterns
- **API Reference**: See [api-reference.md](api-reference.md) for complete method documentation  
- **Troubleshooting**: See [troubleshooting.md](troubleshooting.md) if values aren't updating
- **Advanced Usage**: See [advanced.md](advanced.md) for performance tuning, custom exports